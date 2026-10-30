package completion

import (
	"fmt"
	"strings"

	"github.com/ekkolyth/miso/internal/cli"
	"github.com/ekkolyth/miso/internal/cli/scripting"
	"github.com/ekkolyth/miso/internal/scripts"
	"github.com/ekkolyth/miso/internal/workspace"
)

// upgradeFlags are flags for the upgrade command.
var upgradeFlags = []string{"--local"}

// builtinSet: commands that take package names (no completion for now).
var builtinSet = map[string]bool{
	"add": true, "remove": true, "rm": true,
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

func getCandidates(prev string, _ string, cwd string) []string {
	// Completing first word after "miso" (prev is "miso" or empty)
	if prev == "miso" || prev == "" {
		candidates := cli.BuiltinNames()

		seen := make(map[string]bool)
		for _, c := range candidates {
			seen[c] = true
		}
		for _, n := range projectScriptNames(cwd) {
			if !seen[n] {
				seen[n] = true
				candidates = append(candidates, n)
			}
		}
		return candidates
	}

	// add, remove: no package name completion (deferred)
	if builtinSet[prev] {
		return nil
	}

	// upgrade: complete --local
	if prev == "upgrade" {
		return upgradeFlags
	}

	// After "run" or "dev", or after a script name (run s1 s2): complete script names
	return projectScriptNames(cwd)
}

// root scripts, plus the scripts of the member cwd is in, since those run from there
func projectScriptNames(cwd string) []string {
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
	// members are found whatever the root's mode
	members, _ := workspace.DiscoverMembers(root, cfg)
	member, inMember := workspace.FromCWD(cwd, members)
	if !inMember {
		return names
	}
	memberNames, err := scripting.ListNames(member.Dir, workspace.EffectiveConfig(cfg, member))
	if err != nil {
		return names
	}
	seen := make(map[string]bool)
	for _, n := range names {
		seen[n] = true
	}
	for _, n := range memberNames {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	return names
}

func ScriptBash() string {
	return scripts.Bash
}

func ScriptZsh() string {
	return scripts.Zsh
}

func ScriptFish() string {
	return scripts.Fish
}
