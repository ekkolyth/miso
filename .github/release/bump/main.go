package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type packageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func main() {
	level := flag.String("level", "patch", "version bump level: patch, minor, or major")
	dryRun := flag.Bool("dry-run", false, "print the new version without writing changes")
	flag.Parse()

	info, err := readPackage()
	if err != nil {
		fmt.Fprintln(os.Stderr, "read package.json:", err)
		os.Exit(1)
	}

	current := strings.TrimSpace(info.Version)
	if current == "" {
		fmt.Fprintln(os.Stderr, "package.json: version is empty")
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
	if err := writePackage(info); err != nil {
		fmt.Fprintln(os.Stderr, "write package.json:", err)
		os.Exit(1)
	}

	fmt.Println(next)
}

func readPackage() (*packageInfo, error) {
	data, err := os.ReadFile(".github/package.json")
	if err != nil {
		return nil, err
	}
	var info packageInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func writePackage(info *packageInfo) error {
	// Read the full package.json to preserve other fields
	fullData, err := os.ReadFile(".github/package.json")
	if err != nil {
		return err
	}
	
	var fullPkg map[string]interface{}
	if err := json.Unmarshal(fullData, &fullPkg); err != nil {
		return err
	}
	
	// Update only the version field
	fullPkg["version"] = info.Version
	
	data, err := json.MarshalIndent(fullPkg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(".github/package.json", data, 0o644)
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

