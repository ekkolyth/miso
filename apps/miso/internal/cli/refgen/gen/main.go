// Command gen writes the CLI embed copy and the docs reference pages from
// the @ekkolyth/miso-content sources. Run via `go generate ./...` in apps/miso.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ekkolyth/miso/internal/cli/refgen"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	root, err := refgen.RepoRoot(wd)
	if err != nil {
		fail(err)
	}
	bodies, err := refgen.LoadSourceBodies(root)
	if err != nil {
		fail(err)
	}
	if err := refgen.CheckCorrespondence(bodies); err != nil {
		fail(err)
	}

	embedDir := filepath.Join(root, "apps", "miso", "internal", "cli", "reference", "content")
	if err := refgen.WriteEmbedCopy(embedDir, bodies); err != nil {
		fail(err)
	}

	docsDir := filepath.Join(root, "apps", "docs", "content", "reference")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "commands.mdx"), []byte(refgen.CommandsDoc(bodies)), 0o644); err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "reserved-keywords.mdx"), []byte(refgen.ReservedKeywordsDoc()), 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "refgen:", err)
	os.Exit(1)
}
