package completion

import (
	"fmt"
	"strings"

	"github.com/ekkolyth/miso/internal/cli"
	"github.com/ekkolyth/miso/internal/cli/scripting"
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
	return `# Miso shell completion for bash
# Install: Add to ~/.bashrc and restart your shell, or run in current session:
#   eval "$(miso completion bash)"

_miso_completion() {
  local cur prev
  cur="${COMP_WORDS[COMP_CWORD]}"
  if [ $COMP_CWORD -ge 1 ]; then
    prev="${COMP_WORDS[COMP_CWORD-1]}"
  else
    prev="miso"
  fi
  COMPREPLY=($(miso __complete miso "$cur" "$prev" 2>/dev/null))
}

complete -F _miso_completion miso
`
}

// ScriptZsh returns the zsh completion script.
func ScriptZsh() string {
	return `# Miso shell completion for zsh
# Install: Add to ~/.zshrc and restart your shell, or run in current session:
#   eval "$(miso completion zsh)"

_miso() {
  local cur="${words[CURRENT]}"
  local prev="${words[CURRENT-1]}"
  local -a completions
  completions=("${(f)$(miso __complete miso "$cur" "$prev" 2>/dev/null)}")
  compadd -a completions
}

compdef _miso miso
`
}

// ScriptFish returns the fish completion script.
func ScriptFish() string {
	// Fish: we need to pass the command line. commandline -cp gives the line.
	// We'll use a wrapper that invokes miso __complete with the right args.
	// Fish complete -c miso -a '(miso __complete (commandline -cp))' doesn't work well
	// because we need word-by-word. Use a fish function that parses commandline.
	return `# Miso shell completion for fish
# Install: miso completion fish | source
# Or: eval (miso completion fish)

function __miso_complete
  set -l cl (commandline -cp)
  set -l tokens (string split " " -- $cl)
  set -l cur ""
  set -l prev "miso"
  if test (count $tokens) -ge 2
    set cur $tokens[-1]
    set prev $tokens[-2]
  else if test (count $tokens) -eq 1
    set prev $tokens[1]
  end
  miso __complete miso "$cur" "$prev"
end

complete -c miso -a '(__miso_complete)'
`
}

