package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type packageInfo struct {
	Version string `json:"version"`
}

func main() {
	// Find package.json relative to this source file's location
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	// version/ -> release/ -> .github/ -> project root
	projectRoot := filepath.Join(dir, "..", "..", "..")
	packageJSONPath := filepath.Join(projectRoot, ".github", "package.json")
	
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read package.json:", err)
		os.Exit(1)
	}

	var info packageInfo
	if err := json.Unmarshal(data, &info); err != nil {
		fmt.Fprintln(os.Stderr, "parse package.json:", err)
		os.Exit(1)
	}

	if info.Version == "" {
		fmt.Fprintln(os.Stderr, "package.json: version is empty")
		os.Exit(1)
	}

	fmt.Println(info.Version)
}

