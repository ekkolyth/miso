package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type releaseInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func main() {
	level := flag.String("level", "patch", "version bump level: patch, minor, or major")
	dryRun := flag.Bool("dry-run", false, "print the new version without writing changes")
	flag.Parse()

	info, err := readRelease()
	if err != nil {
		fmt.Fprintln(os.Stderr, "read release.json:", err)
		os.Exit(1)
	}

	current := strings.TrimSpace(info.Version)
	if current == "" {
		fmt.Fprintln(os.Stderr, "release.json: version is empty")
		os.Exit(1)
	}

	next, err := bumpVersion(current, *level)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *dryRun {
		fmt.Println(next)
		return
	}

	info.Version = next
	if err := writeRelease(info); err != nil {
		fmt.Fprintln(os.Stderr, "write release.json:", err)
		os.Exit(1)
	}

	fmt.Println(next)
}

func readRelease() (*releaseInfo, error) {
	data, err := os.ReadFile(".github/release/release.json")
	if err != nil {
		return nil, err
	}
	var info releaseInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func writeRelease(info *releaseInfo) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(".github/release/release.json", data, 0o644)
}

func bumpVersion(current, level string) (string, error) {
	parts := strings.Split(current, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid semver %q (expected format X.Y.Z)", current)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid major version %q: %w", parts[0], err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid minor version %q: %w", parts[1], err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid patch version %q: %w", parts[2], err)
	}

	switch strings.ToLower(level) {
	case "patch":
		patch++
	case "minor":
		minor++
		patch = 0
	case "major":
		major++
		minor = 0
		patch = 0
	default:
		return "", fmt.Errorf("unknown level %q (expected patch, minor, or major)", level)
	}

	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}

