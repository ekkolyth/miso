package commands

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ── Mock HTTP client ──────────────────────────────────────────────────────────

type mockHTTPClient struct {
	responses map[string]*http.Response
	errors    map[string]error
}

func (m *mockHTTPClient) Get(url string) (*http.Response, error) {
	if err, ok := m.errors[url]; ok {
		return nil, err
	}
	if resp, ok := m.responses[url]; ok {
		return resp, nil
	}
	return nil, fmt.Errorf("mockHTTPClient: no response registered for %q", url)
}

func newMockResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// makeTarGz builds an in-memory .tar.gz containing a single file named `name`.
func makeTarGz(name, content string) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	data := []byte(content)
	_ = tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0755,
		Size: int64(len(data)),
	})
	_, _ = tw.Write(data)
	_ = tw.Close()
	_ = gw.Close()
	return buf.Bytes()
}

// ── fetchLatestVersion ────────────────────────────────────────────────────────

func TestFetchLatestVersion_success(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"tag_name": "v0.4.0"})
	client := &mockHTTPClient{
		responses: map[string]*http.Response{
			githubLatestURL: newMockResponse(200, string(body)),
		},
	}
	version, err := fetchLatestVersion(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "0.4.0" {
		t.Errorf("expected %q, got %q", "0.4.0", version)
	}
}

func TestFetchLatestVersion_stripsV(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"tag_name": "v1.2.3"})
	client := &mockHTTPClient{
		responses: map[string]*http.Response{
			githubLatestURL: newMockResponse(200, string(body)),
		},
	}
	version, err := fetchLatestVersion(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "1.2.3" {
		t.Errorf("expected %q, got %q", "1.2.3", version)
	}
}

func TestFetchLatestVersion_httpError(t *testing.T) {
	client := &mockHTTPClient{
		errors: map[string]error{
			githubLatestURL: fmt.Errorf("connection refused"),
		},
	}
	_, err := fetchLatestVersion(client)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchLatestVersion_non200(t *testing.T) {
	client := &mockHTTPClient{
		responses: map[string]*http.Response{
			githubLatestURL: newMockResponse(404, "not found"),
		},
	}
	_, err := fetchLatestVersion(client)
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
}

// ── platformTarget ────────────────────────────────────────────────────────────

func TestPlatformTarget_knownPlatforms(t *testing.T) {
	cases := []struct {
		goos, goarch string
		want         string
	}{
		{"darwin", "amd64", "darwin_amd64"},
		{"darwin", "arm64", "darwin_arm64"},
		{"linux", "amd64", "linux_amd64"},
		{"linux", "arm64", "linux_arm64"},
	}
	for _, tc := range cases {
		t.Run(tc.goos+"/"+tc.goarch, func(t *testing.T) {
			got, err := platformTarget(tc.goos, tc.goarch)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestPlatformTarget_unsupported(t *testing.T) {
	cases := [][2]string{
		{"windows", "amd64"},
		{"windows", "arm64"},
		{"freebsd", "amd64"},
		{"linux", "mips"},
	}
	for _, tc := range cases {
		t.Run(tc[0]+"/"+tc[1], func(t *testing.T) {
			_, err := platformTarget(tc[0], tc[1])
			if err == nil {
				t.Errorf("expected error for %s/%s, got nil", tc[0], tc[1])
			}
		})
	}
}

// ── buildDownloadURL ──────────────────────────────────────────────────────────

func TestBuildDownloadURL(t *testing.T) {
	got := buildDownloadURL("miso", "0.4.0", "darwin_arm64")
	want := "https://github.com/ekkolyth/miso/releases/download/v0.4.0/miso_0.4.0_darwin_arm64.tar.gz"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestBuildDownloadURL_misox(t *testing.T) {
	got := buildDownloadURL("misox", "0.4.0", "darwin_arm64")
	want := "https://github.com/ekkolyth/miso/releases/download/v0.4.0/misox_0.4.0_darwin_arm64.tar.gz"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// ── extractBinaryFromTarGz ────────────────────────────────────────────────────

func TestExtractBinaryFromTarGz_success(t *testing.T) {
	archiveData := makeTarGz("miso-darwin-arm64", "fake binary")
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "miso")

	err := extractBinaryFromTarGz(bytes.NewReader(archiveData), "miso-darwin-arm64", destPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("could not read extracted file: %v", err)
	}
	if string(data) != "fake binary" {
		t.Errorf("extracted content mismatch: %q", string(data))
	}
	info, _ := os.Stat(destPath)
	if info.Mode()&0111 == 0 {
		t.Error("extracted binary is not executable")
	}
}

func TestExtractBinaryFromTarGz_missingEntry(t *testing.T) {
	archiveData := makeTarGz("miso-linux-amd64", "fake binary")
	err := extractBinaryFromTarGz(
		bytes.NewReader(archiveData),
		"miso-darwin-arm64",
		filepath.Join(t.TempDir(), "miso"),
	)
	if err == nil {
		t.Fatal("expected error for missing entry, got nil")
	}
}

// ── installBinary ─────────────────────────────────────────────────────────────

func TestInstallBinary_sameDevice(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new-miso")
	dst := filepath.Join(dir, "miso")

	if err := os.WriteFile(src, []byte("new content"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old content"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := installBinary(src, dst); err != nil {
		t.Fatalf("installBinary error: %v", err)
	}

	data, _ := os.ReadFile(dst)
	if string(data) != "new content" {
		t.Errorf("expected %q, got %q", "new content", string(data))
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("source file should have been removed after install")
	}
}

// ── upgradeOneBinary ─────────────────────────────────────────────────────────

func TestUpgradeOneBinary_extractsCorrectEntry(t *testing.T) {
	// Build a fake archive for "miso-darwin-arm64"
	archiveData := makeTarGz("miso-darwin-arm64", "miso content")

	version := "0.4.0"
	target := "darwin_arm64"
	binaryName := "miso"
	url := buildDownloadURL(binaryName, version, target)
	entryName := binaryName + "-" + strings.ReplaceAll(target, "_", "-")

	client := &mockHTTPClient{
		responses: map[string]*http.Response{
			url: {
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(archiveData)),
			},
		},
	}

	destDir := t.TempDir()
	destPath := filepath.Join(destDir, binaryName)
	// pre-create destination so installBinary can replace it
	if err := os.WriteFile(destPath, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := upgradeOneBinary(client, binaryName, version, target, entryName, destPath); err != nil {
		t.Fatalf("upgradeOneBinary error: %v", err)
	}

	data, _ := os.ReadFile(destPath)
	if string(data) != "miso content" {
		t.Errorf("expected %q after upgrade, got %q", "miso content", string(data))
	}
}

// ── Integration: current platform is supported ────────────────────────────────

func TestUpgrade_currentPlatformIsSupported(t *testing.T) {
	_, err := platformTarget(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Skipf("current platform %s/%s not supported: %v", runtime.GOOS, runtime.GOARCH, err)
	}
}
