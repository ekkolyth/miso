package commands

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const githubLatestURL = "https://api.github.com/repos/ekkolyth/miso/releases/latest"

// for test injection without a real HTTP server
type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

// githubRelease is the subset of the GitHub Releases API response we care about.
type githubRelease struct {
	TagName string `json:"tag_name"`
}

// fetchLatestVersion queries the GitHub Releases API and returns the latest
// version string with the leading "v" stripped (e.g. "0.4.0").
func fetchLatestVersion(client HTTPClient) (string, error) {
	resp, err := client.Get(githubLatestURL)
	if err != nil {
		return "", fmt.Errorf("fetch latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("decode GitHub API response: %w", err)
	}
	if release.TagName == "" {
		return "", fmt.Errorf("GitHub API response missing tag_name")
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}

// platformTarget maps GOOS/GOARCH to the naming used in GitHub release archives
// (e.g. "darwin_arm64"). Returns an error for unsupported platforms.
func platformTarget(goos, goarch string) (string, error) {
	supported := map[string]map[string]bool{
		"darwin": {"amd64": true, "arm64": true},
		"linux":  {"amd64": true, "arm64": true},
	}
	archs, ok := supported[goos]
	if !ok {
		if goos == "windows" {
			return "", fmt.Errorf(
				"automatic upgrade is not supported on Windows\n" +
					"Please reinstall manually: https://misojs.dev/install#windows",
			)
		}
		return "", fmt.Errorf("unsupported OS: %s", goos)
	}
	if !archs[goarch] {
		return "", fmt.Errorf("unsupported architecture: %s/%s", goos, goarch)
	}
	return goos + "_" + goarch, nil
}

// buildDownloadURL constructs the GitHub release archive URL.
// binaryName is "miso" or "misox". version is without the "v" prefix. target is e.g. "darwin_arm64".
func buildDownloadURL(binaryName, version, target string) string {
	return fmt.Sprintf(
		"https://github.com/ekkolyth/miso/releases/download/v%s/%s_%s_%s.tar.gz",
		version, binaryName, version, target,
	)
}

// extractBinaryFromTarGz reads a .tar.gz archive from r, finds the entry named
// entryName, and writes it to destPath with executable permissions.
func extractBinaryFromTarGz(r io.Reader, entryName, destPath string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("open gzip: %w", err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if hdr.Name != entryName {
			continue
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != 0 {
			return fmt.Errorf("archive entry %q is not a regular file (type %d)", entryName, hdr.Typeflag)
		}
		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("create dest file: %w", err)
		}
		if _, err := io.Copy(f, tr); err != nil {
			_ = f.Close()
			return fmt.Errorf("write binary: %w", err)
		}
		return f.Close()
	}
	return fmt.Errorf("binary %q not found in archive", entryName)
}

// installBinary atomically replaces dst with src.
// Tries os.Rename first (atomic, same device); falls back to copy+rename on cross-device.
func installBinary(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	// Cross-device fallback: copy to a temp file next to dst, then rename
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.CreateTemp(filepath.Dir(dst), ".miso-upgrade-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpDst := out.Name()
	if err := out.Chmod(0755); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpDst)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmpDst)
		return fmt.Errorf("copy binary: %w", err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpDst)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpDst, dst); err != nil {
		_ = os.Remove(tmpDst)
		return fmt.Errorf("rename to dest: %w", err)
	}
	_ = os.Remove(src)
	return nil
}

// upgradeOneBinary downloads the archive for one binary (miso or misox), extracts it,
// and installs it to destPath.
func upgradeOneBinary(client HTTPClient, binaryName, version, target, entryName, destPath string) error {
	url := buildDownloadURL(binaryName, version, target)
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", binaryName, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s failed: HTTP %d", binaryName, resp.StatusCode)
	}

	tmpDir, err := os.MkdirTemp("", "miso-upgrade-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	const maxBinaryBytes = 100 * 1024 * 1024 // 100 MB
	tmpBinary := filepath.Join(tmpDir, binaryName)
	if err := extractBinaryFromTarGz(io.LimitReader(resp.Body, maxBinaryBytes), entryName, tmpBinary); err != nil {
		return fmt.Errorf("extract %s: %w", binaryName, err)
	}
	if err := installBinary(tmpBinary, destPath); err != nil {
		return fmt.Errorf("install %s: %w", binaryName, err)
	}
	return nil
}

// Upgrade downloads the latest miso (and misox) binary from GitHub Releases
// and replaces the current binary in-place. args is reserved for future flags.
func Upgrade(_ []string) error {
	client := &http.Client{Timeout: 60 * time.Second}

	// 1. Detect platform
	target, err := platformTarget(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}

	// 2. Fetch latest version
	fmt.Println("Checking for latest version...")
	version, err := fetchLatestVersion(client)
	if err != nil {
		return fmt.Errorf("could not fetch latest version: %w", err)
	}

	// 3. Locate current binary (resolve symlinks)
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current binary: %w", err)
	}
	currentBinary, err = filepath.EvalSymlinks(currentBinary)
	if err != nil {
		return fmt.Errorf("resolve symlinks: %w", err)
	}
	binaryDir := filepath.Dir(currentBinary)
	targetHyphen := strings.ReplaceAll(target, "_", "-")

	// 4. Upgrade miso
	fmt.Printf("Downloading miso v%s (%s)...\n", version, target)
	misoEntry := "miso-" + targetHyphen
	if err := upgradeOneBinary(client, "miso", version, target, misoEntry, currentBinary); err != nil {
		return err
	}

	// 5. Upgrade misox if it exists alongside miso
	misoxPath := filepath.Join(binaryDir, "misox")
	if _, err := os.Stat(misoxPath); err == nil {
		fmt.Printf("Downloading misox v%s (%s)...\n", version, target)
		misoxEntry := "misox-" + targetHyphen
		if err := upgradeOneBinary(client, "misox", version, target, misoxEntry, misoxPath); err != nil {
			// Non-fatal: miso upgrade succeeded, misox upgrade failed
			fmt.Printf("warning: could not upgrade misox: %v\n", err)
		}
	}

	fmt.Printf("✓ miso upgraded to v%s\n", version)
	return nil
}
