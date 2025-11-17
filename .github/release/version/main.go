package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type packageInfo struct {
	Version string `json:"version"`
}

func main() {
	data, err := os.ReadFile(".github/release/package.json")
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

