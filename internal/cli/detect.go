package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var lockfileMap = map[string]string{
	"bun.lockb":         "bun",
	"bun.lock":          "bun", 
	"package-lock.json": "npm",
	"pnpm-lock.yaml":    "pnpm",
	"yarn.lock":         "yarn",
}

func DetectManager(directoryPath string) (string, error) {
	found := map[string][]string{}

	for lockfile, name := range lockfileMap {
		p := filepath.Join(directoryPath, lockfile)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			found[name] = append(found[name], lockfile)
		}
	}

	switch len(found) {
	case 0:
		return "", errors.New("no lockfile found (checked bun.lockb, package-lock.json, pnpm-lock.yaml, yarn.lock)")
	case 1:
		for k := range found {
			return k, nil
		}
	default:
		s := "multiple lockfiles detected; resolve conflict:\n"
		for k, files := range found {
			s += fmt.Sprintf("  %s: %v\n", k, files)
		}
		return "", errors.New(s)
	}
	return "", errors.New("unreachable")
}



