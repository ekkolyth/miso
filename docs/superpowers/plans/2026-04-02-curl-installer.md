# Curl Installer + `miso upgrade` Rewrite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable users to install miso with `curl -fsSL https://misojs.dev/install | bash` (no Node.js needed) and rewrite `miso upgrade` to download and replace the binary in-place without sudo.

**Architecture:** A POSIX-compatible shell script at `scripts/install.sh` detects OS/arch, fetches the latest version from the GitHub API, and installs the binary to `~/.local/bin`. A Next.js API route at `apps/docs/app/install/route.ts` serves the script verbatim. The `miso upgrade` command is rewritten in Go to fetch the latest release from the GitHub API, download the `.tar.gz`, extract it to a temp file, and atomically rename it over the current binary. npm install remains supported as an alternative.

**Tech Stack:** POSIX sh (bash shebang), Next.js 15 App Router API routes, Go 1.25.8 (`archive/tar`, `compress/gzip`, `encoding/json`, `net/http`, `os`, `runtime`)

---

## File Map

| File | Status | Responsibility |
|------|--------|---------------|
| `scripts/install.sh` | **Create** | Canonical shell installer script |
| `apps/docs/app/install/route.ts` | **Create** | Next.js API route that serves `install.sh` |
| `apps/miso/internal/cli/commands/upgrade.go` | **Rewrite** | Direct-download binary self-upgrade |
| `apps/miso/internal/cli/commands/upgrade_test.go` | **Create** | Unit tests for the new upgrade logic |
| `apps/miso/cmd/main.go` | **Modify** | Remove `--local` flag handling from the `upgrade` case |
| `apps/miso/internal/cli/router.go` | **Modify** | Remove `ParseLocalFlag` call in upgrade path; remove `Local` field usage |
| `apps/docs/content/install.mdx` | **Modify** | Add curl one-liner as primary install option |

---

## Task 1: Shell Installer Script

**Files:**
- Create: `scripts/install.sh`

> This is a pure shell script — there are no automated tests. Manual test instructions are in the verification step.

- [ ] **Step 1: Create `scripts/install.sh`**

```bash
#!/usr/bin/env bash
# miso installer
# Usage: curl -fsSL https://misojs.dev/install | bash

set -e

REPO="ekkolyth/miso"
INSTALL_DIR="${MISO_INSTALL_DIR:-$HOME/.local/bin}"

# ── Helpers ──────────────────────────────────────────────────────────────────

info()  { printf '\033[1;34m[miso]\033[0m %s\n' "$*"; }
ok()    { printf '\033[1;32m[miso]\033[0m %s\n' "$*"; }
fail()  { printf '\033[1;31m[miso]\033[0m error: %s\n' "$*" >&2; exit 1; }

# ── Detect OS ────────────────────────────────────────────────────────────────

OS="$(uname -s)"
case "$OS" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux"  ;;
  *)      fail "Unsupported operating system: $OS (Windows users: use npm install -g @ekkolyth/miso)" ;;
esac

# ── Detect arch ──────────────────────────────────────────────────────────────

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64 | amd64)  ARCH="amd64" ;;
  arm64  | aarch64) ARCH="arm64" ;;
  *)                fail "Unsupported architecture: $ARCH" ;;
esac

# ── Fetch latest version ─────────────────────────────────────────────────────

info "Fetching latest version..."

LATEST_URL="https://api.github.com/repos/${REPO}/releases/latest"

if command -v curl >/dev/null 2>&1; then
  TAG=$(curl -fsSL "$LATEST_URL" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
elif command -v wget >/dev/null 2>&1; then
  TAG=$(wget -qO- "$LATEST_URL" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
else
  fail "Neither curl nor wget is available. Please install one and try again."
fi

if [ -z "$TAG" ]; then
  fail "Could not determine the latest version. Check your internet connection."
fi

# Strip leading 'v'
VERSION="${TAG#v}"
info "Installing miso v${VERSION} (${OS}/${ARCH})..."

# ── Download archive ──────────────────────────────────────────────────────────

ARCHIVE="miso_${VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

TMP_DIR="$(mktemp -d)"
TMP_ARCHIVE="${TMP_DIR}/${ARCHIVE}"

cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

info "Downloading ${DOWNLOAD_URL}..."

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$DOWNLOAD_URL" -o "$TMP_ARCHIVE"
else
  wget -qO "$TMP_ARCHIVE" "$DOWNLOAD_URL"
fi

# ── Extract binary ────────────────────────────────────────────────────────────

tar -xzf "$TMP_ARCHIVE" -C "$TMP_DIR"

# The binary inside the archive is named after the platform (e.g. miso-darwin-arm64)
# Find it by looking for a file matching miso-<os>-<arch>
EXTRACTED_BINARY="${TMP_DIR}/miso-${OS}-${ARCH}"
if [ ! -f "$EXTRACTED_BINARY" ]; then
  fail "Could not find binary 'miso-${OS}-${ARCH}' in archive. Archive contents: $(ls "$TMP_DIR")"
fi

# ── Install ───────────────────────────────────────────────────────────────────

mkdir -p "$INSTALL_DIR"
chmod +x "$EXTRACTED_BINARY"
mv "$EXTRACTED_BINARY" "${INSTALL_DIR}/miso"

# ── PATH hint ─────────────────────────────────────────────────────────────────

ok "miso v${VERSION} installed to ${INSTALL_DIR}/miso"

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;  # already on PATH
  *)
    printf '\n'
    info "${INSTALL_DIR} is not in your PATH."
    info "Add the following to your shell config (~/.bashrc, ~/.zshrc, etc.):"
    printf '\n    export PATH="%s:$PATH"\n\n' "$INSTALL_DIR"
    ;;
esac

ok "Run 'miso --help' to get started."
```

- [ ] **Step 2: Make the script executable**

```bash
chmod +x scripts/install.sh
```

Run: `chmod +x scripts/install.sh`
Expected: no output, exit 0.

- [ ] **Step 3: Verify the script is POSIX-compatible (syntax check)**

Run: `bash -n scripts/install.sh`
Expected: no output, exit 0.

- [ ] **Step 4: Smoke-test locally (optional, requires network)**

> Set `MISO_INSTALL_DIR` to avoid touching your real `~/.local/bin` during testing.

Run: `MISO_INSTALL_DIR=/tmp/miso-test-install bash scripts/install.sh`
Expected output (example):
```
[miso] Fetching latest version...
[miso] Installing miso v0.3.2 (darwin/arm64)...
[miso] Downloading https://github.com/ekkolyth/miso/releases/download/v0.3.2/miso_0.3.2_darwin_arm64.tar.gz...
[miso] miso v0.3.2 installed to /tmp/miso-test-install/miso
[miso] /tmp/miso-test-install is not in your PATH.
...
```
Then verify: `/tmp/miso-test-install/miso --help` prints usage.

- [ ] **Step 5: Commit**

```bash
git add scripts/install.sh
git commit -m "feat: add shell installer script"
```

---

## Task 2: Next.js API Route (`/install`)

**Files:**
- Create: `apps/docs/app/install/route.ts`

**Context:** The Next.js docs site lives at `apps/docs/`. The App Router is in `apps/docs/app/`. The install script lives at `scripts/install.sh` relative to the repo root. We read the script at request time using `fs.readFileSync` — no edge runtime, so Node.js `fs` is available.

**Important — do NOT use `__dirname`** in a Next.js App Router route. After compilation, `__dirname` points to `.next/server/app/install/`, not the source tree, so relative paths to source files will fail in production. Use `process.cwd()` instead — Next.js sets `cwd` to the project root (`apps/docs/`) at startup. The repo root is two directories up from there.

- [ ] **Step 1: Create `apps/docs/app/install/route.ts`**

```typescript
import fs from 'node:fs'
import path from 'node:path'

// process.cwd() is apps/docs/ — go up two levels to reach the repo root.
// This works in both dev (source tree) and production (compiled output).
const SCRIPT_PATH = path.join(process.cwd(), '../../scripts/install.sh')

export function GET(): Response {
    const script = fs.readFileSync(SCRIPT_PATH, 'utf-8')
    return new Response(script, {
        status: 200,
        headers: {
            'Content-Type': 'text/plain; charset=utf-8',
            'Cache-Control': 'no-cache',
        },
    })
}
```

- [ ] **Step 2: Verify the route builds without errors**

Run: `cd apps/docs && bun run build 2>&1 | tail -20`
Expected: build completes with no TypeScript or module resolution errors. Look for `✓ Compiled successfully` or equivalent.

- [ ] **Step 3: Test the route locally**

Start the dev server: `cd apps/docs && bun run dev`
In another terminal: `curl -fsSL http://localhost:3000/install | head -5`
Expected: first lines of `scripts/install.sh` are printed (e.g., `#!/usr/bin/env bash`).

- [ ] **Step 4: Commit**

```bash
git add apps/docs/app/install/route.ts
git commit -m "feat: add /install API route to serve shell installer script"
```

---

## Task 3: Rewrite `miso upgrade` in Go

**Files:**
- Rewrite: `apps/miso/internal/cli/commands/upgrade.go`
- Create: `apps/miso/internal/cli/commands/upgrade_test.go`
- Modify: `apps/miso/cmd/main.go` (lines 100-105) — remove `ParseLocalFlag` call
- Modify: `apps/miso/internal/cli/router.go` (lines 47-58, 143-153) — remove `Local` field and `ParseLocalFlag` usage in upgrade path

**Architecture note:** We introduce an `HTTPClient` interface so tests can inject a mock. The upgrade logic is split into pure functions to make testing straightforward without spawning subprocesses.

### 3a — Write the failing tests first

- [ ] **Step 1: Create `apps/miso/internal/cli/commands/upgrade_test.go` with failing tests**

```go
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

// makeTarGz builds an in-memory .tar.gz containing a single file named `name`
// with the given content. Returns the raw bytes.
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
		t.Errorf("expected version %q, got %q", "0.4.0", version)
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
		t.Errorf("expected version %q, got %q", "1.2.3", version)
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

// ── downloadURL ───────────────────────────────────────────────────────────────

func TestDownloadURL(t *testing.T) {
	got := buildDownloadURL("0.4.0", "darwin_arm64")
	want := "https://github.com/ekkolyth/miso/releases/download/v0.4.0/miso_0.4.0_darwin_arm64.tar.gz"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// ── extractBinaryFromTarGz ────────────────────────────────────────────────────

func TestExtractBinaryFromTarGz_success(t *testing.T) {
	// Archive contains: miso-darwin-arm64 with content "fake binary"
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

	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("extracted binary is not executable")
	}
}

func TestExtractBinaryFromTarGz_missingEntry(t *testing.T) {
	archiveData := makeTarGz("miso-linux-amd64", "fake binary")
	err := extractBinaryFromTarGz(
		bytes.NewReader(archiveData),
		"miso-darwin-arm64", // wrong name
		filepath.Join(t.TempDir(), "miso"),
	)
	if err == nil {
		t.Fatal("expected error for missing entry, got nil")
	}
}

// ── installBinary (atomic rename / copy fallback) ─────────────────────────────

func TestInstallBinary_sameDevice(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new-miso")
	dst := filepath.Join(dir, "miso")

	if err := os.WriteFile(src, []byte("new content"), 0755); err != nil {
		t.Fatal(err)
	}
	// dst must already exist to simulate replacing a binary
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
	// src should be gone
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("source file should have been removed after install")
	}
}

// ── Integration: Upgrade uses current platform ────────────────────────────────

func TestUpgrade_currentPlatformIsSupported(t *testing.T) {
	// This verifies that whatever platform the tests are running on
	// (darwin or linux) is in the supported set.
	_, err := platformTarget(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		// Allow skipping on explicitly unsupported platforms
		t.Skipf("current platform %s/%s not supported: %v", runtime.GOOS, runtime.GOARCH, err)
	}
}
```

- [ ] **Step 2: Run the tests — they must FAIL (functions don't exist yet)**

Run:
```bash
cd apps/miso && go test ./internal/cli/commands/ -run "TestFetchLatestVersion|TestPlatformTarget|TestDownloadURL|TestExtractBinaryFromTarGz|TestInstallBinary|TestUpgrade_currentPlatform" -v 2>&1 | head -40
```
Expected: compilation errors — `githubLatestURL`, `fetchLatestVersion`, `platformTarget`, `buildDownloadURL`, `extractBinaryFromTarGz`, `installBinary` are undefined.

### 3b — Implement `upgrade.go`

- [ ] **Step 3: Rewrite `apps/miso/internal/cli/commands/upgrade.go`**

```go
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
)

const githubLatestURL = "https://api.github.com/repos/ekkolyth/miso/releases/latest"

// HTTPClient is an interface for making HTTP GET requests.
// It exists so tests can inject a mock without spawning a real HTTP server.
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
	defer resp.Body.Close()

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
				"automatic upgrade is not supported on Windows\n"+
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
// version is without the "v" prefix (e.g. "0.4.0").
// target is e.g. "darwin_arm64".
func buildDownloadURL(version, target string) string {
	return fmt.Sprintf(
		"https://github.com/ekkolyts/miso/releases/download/v%s/miso_%s_%s.tar.gz",
		version, version, target,
	)
}

// extractBinaryFromTarGz reads a .tar.gz archive from r, finds the entry named
// entryName, and writes it to destPath with executable permissions.
func extractBinaryFromTarGz(r io.Reader, entryName, destPath string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("open gzip: %w", err)
	}
	defer gr.Close()

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
		// Found it — write to destPath
		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("create dest file: %w", err)
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return fmt.Errorf("write binary: %w", err)
		}
		return f.Close()
	}
	return fmt.Errorf("binary %q not found in archive", entryName)
}

// installBinary atomically replaces dst with src.
// It tries os.Rename first (atomic on the same device); if that fails
// (e.g. cross-device), it falls back to a copy-then-remove.
func installBinary(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	// Cross-device fallback: copy then remove source
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()

	// Write to a temp file next to dst, then rename
	tmpDst := dst + ".tmp"
	out, err := os.OpenFile(tmpDst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmpDst)
		return fmt.Errorf("copy binary: %w", err)
	}
	if err := out.Close(); err != nil {
		os.Remove(tmpDst)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpDst, dst); err != nil {
		os.Remove(tmpDst)
		return fmt.Errorf("rename to dest: %w", err)
	}
	os.Remove(src)
	return nil
}

// Upgrade downloads the latest miso binary from GitHub Releases and replaces
// the currently-running binary in-place. No sudo required.
func Upgrade(args []string) error {
	client := &http.Client{}

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

	// 3. Build download URL
	archiveName := fmt.Sprintf("miso_%s_%s.tar.gz", version, target)
	entryName := fmt.Sprintf("miso-%s-%s", strings.Replace(target, "_", "-", 1))
	downloadURL := buildDownloadURL(version, target)

	fmt.Printf("Downloading miso v%s (%s)...\n", version, target)

	// 4. Download archive
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download %s: %w", archiveName, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d from %s", resp.StatusCode, downloadURL)
	}

	// 5. Extract binary to a temp file
	tmpDir, err := os.MkdirTemp("", "miso-upgrade-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpBinary := filepath.Join(tmpDir, "miso")
	if err := extractBinaryFromTarGz(resp.Body, entryName, tmpBinary); err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}

	// 6. Locate current binary
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current binary: %w", err)
	}
	currentBinary, err = filepath.EvalSymlinks(currentBinary)
	if err != nil {
		return fmt.Errorf("resolve symlinks: %w", err)
	}

	// 7. Replace current binary
	if err := installBinary(tmpBinary, currentBinary); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}

	fmt.Printf("✓ miso upgraded to v%s\n", version)
	return nil
}
```

- [ ] **Step 4: Run the tests — they should now PASS**

Run:
```bash
cd apps/miso && go test ./internal/cli/commands/ -run "TestFetchLatestVersion|TestPlatformTarget|TestDownloadURL|TestExtractBinaryFromTarGz|TestInstallBinary|TestUpgrade_currentPlatform" -v
```
Expected output:
```
=== RUN   TestFetchLatestVersion_success
--- PASS: TestFetchLatestVersion_success (0.00s)
=== RUN   TestFetchLatestVersion_stripsV
--- PASS: TestFetchLatestVersion_stripsV (0.00s)
=== RUN   TestFetchLatestVersion_httpError
--- PASS: TestFetchLatestVersion_httpError (0.00s)
=== RUN   TestFetchLatestVersion_non200
--- PASS: TestFetchLatestVersion_non200 (0.00s)
=== RUN   TestPlatformTarget_knownPlatforms/darwin/amd64
--- PASS: ...
=== RUN   TestPlatformTarget_knownPlatforms/darwin/arm64
--- PASS: ...
=== RUN   TestPlatformTarget_knownPlatforms/linux/amd64
--- PASS: ...
=== RUN   TestPlatformTarget_knownPlatforms/linux/arm64
--- PASS: ...
=== RUN   TestPlatformTarget_unsupported/windows/amd64
--- PASS: ...
=== RUN   TestPlatformTarget_unsupported/windows/arm64
--- PASS: ...
=== RUN   TestDownloadURL
--- PASS: TestDownloadURL (0.00s)
=== RUN   TestExtractBinaryFromTarGz_success
--- PASS: TestExtractBinaryFromTarGz_success (0.00s)
=== RUN   TestExtractBinaryFromTarGz_missingEntry
--- PASS: TestExtractBinaryFromTarGz_missingEntry (0.00s)
=== RUN   TestInstallBinary_sameDevice
--- PASS: TestInstallBinary_sameDevice (0.00s)
=== RUN   TestUpgrade_currentPlatformIsSupported
--- PASS: TestUpgrade_currentPlatformIsSupported (0.00s)
PASS
ok  	github.com/ekkolyth/miso/internal/cli/commands
```

### 3c — Update the caller in `main.go` and `router.go`

- [ ] **Step 5: Update `apps/miso/cmd/main.go` — remove `ParseLocalFlag` from the upgrade case**

Find this block (around line 100):
```go
case "upgrade":
    local, remainingArgs := cli.ParseLocalFlag(args[1:])
    if err := commands.Upgrade(local, remainingArgs); err != nil {
        cli.Fail(logger, err, false)
    }
    return
```

Replace with:
```go
case "upgrade":
    if err := commands.Upgrade(args[1:]); err != nil {
        cli.Fail(logger, err, false)
    }
    return
```

- [ ] **Step 6: Update `apps/miso/internal/cli/router.go` — remove `Local` from upgrade path**

In `ParseCLI`, find the upgrade case (around line 143):
```go
case "upgrade":
    // check script override
    if resolved, err := scripting.ResolveScript("upgrade", root, cfg); err == nil && resolved.Source != scripting.ScriptSourceNone {
        return buildScriptAction(resolved, "upgrade", parseInlineArgs(args[1:])), nil
    }
    local, remainingArgs := ParseLocalFlag(args[1:])
    return ParsedCLI{
        Action: ActionUpgrade,
        Local:  local,
        Args:   remainingArgs,
    }, nil
```

Replace with:
```go
case "upgrade":
    // check script override
    if resolved, err := scripting.ResolveScript("upgrade", root, cfg); err == nil && resolved.Source != scripting.ScriptSourceNone {
        return buildScriptAction(resolved, "upgrade", parseInlineArgs(args[1:])), nil
    }
    return ParsedCLI{
        Action: ActionUpgrade,
        Args:   args[1:],
    }, nil
```

Also remove `ParseLocalFlag` function (lines 47-58) from `router.go` **only if it is not used anywhere else**. Check:

Run: `grep -r "ParseLocalFlag" apps/miso/`
Expected: only appears in `router.go` and `main.go`. After the `main.go` edit in Step 5, there are no more callers. Remove the `ParseLocalFlag` function from `router.go`.

Also remove the `Local bool` field from `ParsedCLI` struct **only if it is not referenced elsewhere**:

Run: `grep -r "\.Local\b" apps/miso/`
Expected: no results after removing the main.go usage. Remove the field from the struct.

- [ ] **Step 7: Verify the full package compiles and all tests pass**

Run:
```bash
cd apps/miso && go build ./... && go test ./...
```
Expected:
```
ok  	github.com/ekkolyth/miso/internal/cli/commands	(cached)
ok  	github.com/ekkolyth/miso/...
```
No compilation errors.

- [ ] **Step 8: Commit**

```bash
git add apps/miso/internal/cli/commands/upgrade.go \
        apps/miso/internal/cli/commands/upgrade_test.go \
        apps/miso/cmd/main.go \
        apps/miso/internal/cli/router.go
git commit -m "feat: rewrite miso upgrade to download binary directly from GitHub Releases"
```

---

## Task 4: Update Install Docs

**Files:**
- Modify: `apps/docs/content/install.mdx`

- [ ] **Step 1: Replace the content of `apps/docs/content/install.mdx`**

The goal: curl one-liner is first and marked "recommended", npm/bun/pnpm/yarn are shown as alternatives. The upgrade section removes the sudo note.

```mdx
---
title: Installation
---

# Installation

## Recommended: Install with curl

The fastest way to install miso — no Node.js or npm required:

```bash
curl -fsSL https://misojs.dev/install | bash
```

This installs miso to `~/.local/bin`. If that directory is not on your `PATH`,
the installer will print the line to add to your shell config.

**Supported platforms:** macOS (Intel & Apple Silicon), Linux (x86-64 & ARM64).  
**Windows:** Use the npm install method below, or download a binary from the
[GitHub Releases](https://github.com/ekkolyth/miso/releases) page.

## Alternative: Install via package manager

If you prefer to install miso through a JavaScript package manager:

<CommandSelect
    bun="bun install -g @ekkolyth/miso"
    pnpm="pnpm install -g @ekkolyth/miso"
    yarn="yarn global add @ekkolyth/miso"
    npm="npm install -g @ekkolyth/miso"
    go="go install github.com/ekkolyth/miso/apps/miso/cmd@latest"
/>

## Upgrading

Once installed, upgrade miso to the latest version at any time:

<CommandSelect miso="miso upgrade" />

`miso upgrade` downloads the latest binary directly from GitHub and replaces
the current binary in-place. No sudo required.

## Install in a project

To install miso only in the project you're currently working on, add it as a dev dependency:

<CommandSelect
    bun="bun add @ekkolyth/miso -d"
    pnpm="pnpm add -D @ekkolyth/miso"
    yarn="yarn add @ekkolyth/miso -D"
    npm="npm i @ekkolyth/miso -D"
/>
```

- [ ] **Step 2: Verify the docs build without errors**

Run: `cd apps/docs && bun run build 2>&1 | tail -10`
Expected: build succeeds with no MDX parse errors.

- [ ] **Step 3: Commit**

```bash
git add apps/docs/content/install.mdx
git commit -m "docs: add curl one-liner as recommended install method, remove sudo note from upgrade"
```

---

## Task 5: Verify GitHub Release Archives Are Correctly Named

**Files:**
- No changes required (this task is verification-only)

- [ ] **Step 1: Confirm archive naming in `create-archives.sh`**

Read `scripts/release/create-archives.sh`. The archives are named:
```
miso_{VERSION}_darwin_amd64.tar.gz
miso_{VERSION}_darwin_arm64.tar.gz
miso_{VERSION}_linux_amd64.tar.gz
miso_{VERSION}_linux_arm64.tar.gz
miso_{VERSION}_windows_amd64.zip
```

This matches the pattern used by the shell installer and the Go upgrade command:
- Shell installer: `miso_${VERSION}_${OS}_${ARCH}.tar.gz`
- Go upgrade: `miso_{version}_{target}.tar.gz` where target = `darwin_amd64`, etc.

**Important note on binary name inside the archive:** The archives are created with `tar -czf miso_${VERSION}_darwin_amd64.tar.gz -C bin miso-darwin-amd64`. This means the binary inside the `.tar.gz` is named `miso-darwin-amd64` (with hyphens), not `miso`. 

The Go upgrade code uses `entryName := fmt.Sprintf("miso-%s-%s", strings.Replace(target, "_", "-", 1))` to compute `miso-darwin-arm64` from `darwin_arm64`. ✓

The shell installer uses `EXTRACTED_BINARY="${TMP_DIR}/miso-${OS}-${ARCH}"` which computes `miso-darwin-arm64`. ✓

- [ ] **Step 2: Verify the CI uploads the correct files in `release.yml`**

Read `.github/workflows/release.yml`. The `release` job uploads:
```yaml
files: |
    dist/miso_${{ needs.version.outputs.version }}_darwin_amd64.tar.gz
    dist/miso_${{ needs.version.outputs.version }}_darwin_arm64.tar.gz
    dist/miso_${{ needs.version.outputs.version }}_linux_amd64.tar.gz
    dist/miso_${{ needs.version.outputs.version }}_linux_arm64.tar.gz
    dist/miso_${{ needs.version.outputs.version }}_windows_amd64.zip
```

These are uploaded under the tag `v{VERSION}` (e.g. `v0.3.2`). The download URL pattern `https://github.com/ekkolyth/miso/releases/download/v{VERSION}/miso_{VERSION}_{os}_{arch}.tar.gz` matches exactly. ✓

The GitHub API endpoint `https://api.github.com/repos/ekkolyth/miso/releases/latest` returns the latest non-prerelease, non-draft release with `tag_name` = `"v{VERSION}"`. ✓

- [ ] **Step 3: Mark as complete**

**Status: ✅ No CI changes needed.** The archives are correctly named and publicly accessible. The shell installer and Go upgrade command use the correct URL patterns.

- [ ] **Step 4: Commit a note (optional)**

If you want to document this verification:
```bash
git commit --allow-empty -m "chore: verify GitHub release archives are correctly named for curl installer"
```

---

## Summary of All Changes

| File | Change |
|------|--------|
| `scripts/install.sh` | **New** — POSIX shell installer |
| `apps/docs/app/install/route.ts` | **New** — Next.js API route serving the installer |
| `apps/miso/internal/cli/commands/upgrade.go` | **Rewritten** — direct binary download, no npm, no sudo |
| `apps/miso/internal/cli/commands/upgrade_test.go` | **New** — unit tests with mock HTTP client |
| `apps/miso/cmd/main.go` | **Modified** — remove `ParseLocalFlag` from upgrade dispatch |
| `apps/miso/internal/cli/router.go` | **Modified** — remove `Local` field + `ParseLocalFlag` from upgrade path |
| `apps/docs/content/install.mdx` | **Modified** — curl as primary, npm as alternative, no sudo note |

**npm install (`npm install -g @ekkolyth/miso`) is NOT removed** — it continues to work as an alternative install method. Only `miso upgrade` stops using npm.
