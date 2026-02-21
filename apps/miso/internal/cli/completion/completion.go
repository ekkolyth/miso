package completion

import (
	"fmt"
	"strings"

	"github.com/ekkolyth/miso/internal/cli"
	"github.com/ekkolyth/miso/internal/cli/scripting"
	"github.com/ekkolyth/miso/internal/scripts"
)

// BuiltinCommands is the list of Miso built-in commands for completion.
var BuiltinCommands = []string{
	"add", "dev", "i", "init", "install", "misox", "remove", "rm", "run",
	"scripts", "upgrade", "v", "version",
}

// upgradeFlags are flags for the upgrade command.
var upgradeFlags = []string{"--local"}

// builtinSet: commands that take package names (no completion for now).
var builtinSet = map[string]bool{
	"add": true, "remove": true, "rm": true, "misox": true,
}

// Complete receives completion args and outputs matching completions to stdout, one per line.
// For bash -C: args are [__complete, miso, currentWord, prevWord].
func Complete(args []string, cwd string) {
	if len(args) < 2 {
		return
	}

	var cur, prev string
	if len(args) >= 4 {
		cur = args[2]
		prev = args[3]
	} else if len(args) >= 3 {
		cur = args[2]
		prev = args[1] // "miso" when completing first word
	} else {
		prev = args[1]
	}

	cur = strings.TrimSpace(cur)
	prev = strings.TrimSpace(prev)

	candidates := getCandidates(prev, cur, cwd)
	for _, c := range candidates {
		if strings.HasPrefix(c, cur) {
			fmt.Println(c)
		}
	}
}

func getCandidates(prev string, cur string, cwd string) []string {
	// Completing first word after "miso" (prev is "miso" or empty)
	if prev == "miso" || prev == "" {
		candidates := make([]string, 0, len(BuiltinCommands))
		candidates = append(candidates, BuiltinCommands...)

		// Add project scripts when in a project
		if root, err := cli.FindProjectRoot(cwd); err == nil {
			if cfg, err := cli.LoadConfig(root); err == nil {
				if names, err := scripting.ListNames(root, cfg); err == nil {
					seen := make(map[string]bool)
					for _, c := range candidates {
						seen[c] = true
					}
					for _, n := range names {
						if !seen[n] {
							seen[n] = true
							candidates = append(candidates, n)
						}
					}
				}
			}
		}

		return candidates
	}

	// add, remove, misox: no package name completion (deferred)
	if builtinSet[prev] {
		return nil
	}

	// upgrade: complete --local
	if prev == "upgrade" {
		return upgradeFlags
	}

	// After "run" or "dev", or after a script name (run s1 s2): complete script names
	root, err := cli.FindProjectRoot(cwd)
	if err != nil {
		return nil
	}
	cfg, err := cli.LoadConfig(root)
	if err != nil {
		return nil
	}
	names, err := scripting.ListNames(root, cfg)
	if err != nil {
		return nil
	}
	return names
}

// ScriptBash returns the bash completion script.
func ScriptBash() string {
	return scripts.Bash
}

// ScriptZsh returns the zsh completion script.
func ScriptZsh() string {
	return scripts.Zsh
}

// ScriptFish returns the fish completion script.
func ScriptFish() string {
	return scripts.Fish
}

