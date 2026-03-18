#!/usr/bin/env node

/**
 * Miso shell completion installer.
 *
 * Run this once after installing miso to set up shell tab-completion:
 *
 *   node <path-to-miso>/scripts/completion.mjs
 *
 * Or if miso is installed globally via npm:
 *
 *   node "$(npm root -g)/@ekkolyth/miso/scripts/completion.mjs"
 */

import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

if (!process.env.HOME) {
	console.error("Could not determine home directory ($HOME is not set).");
	process.exit(1);
}

const MARKERS = {
	bash: "_miso_completion",
	zsh: "compdef _miso miso",
	fish: "complete -c miso",
};

const SCRIPTS = {
	bash: `
# Miso shell completion
_miso_completion() {
  local cur prev
  cur="\${COMP_WORDS[COMP_CWORD]}"
  if [ $COMP_CWORD -ge 1 ]; then
    prev="\${COMP_WORDS[COMP_CWORD-1]}"
  else
    prev="miso"
  fi
  COMPREPLY=($(miso __complete miso "$cur" "$prev" 2>/dev/null))
}

complete -F _miso_completion miso
`,
	zsh: `
# Miso shell completion
_miso() {
  local cur="\${words[CURRENT]}"
  local prev="\${words[CURRENT-1]}"
  local -a completions
  completions=("\${(f)$(miso __complete miso "$cur" "$prev" 2>/dev/null)}")
  compadd -a completions
}

compdef _miso miso
`,
	fish: `
# Miso shell completion
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
`,
};

function detectShell() {
	const shell = process.env.SHELL ?? "";
	const base = shell.split("/").pop() ?? "";
	if (base.startsWith("bash")) return "bash";
	if (base.startsWith("zsh")) return "zsh";
	if (base.startsWith("fish")) return "fish";
	return null;
}

function getConfigPath(shell) {
	const home = process.env.HOME;
	switch (shell) {
		case "bash":
			return join(home, ".bashrc");
		case "zsh":
			return join(home, ".zshrc");
		case "fish":
			return join(home, ".config", "fish", "config.fish");
		default:
			return null;
	}
}

function isAlreadyInstalled(configPath, shell) {
	if (!existsSync(configPath)) return false;
	return readFileSync(configPath, "utf8").includes(MARKERS[shell]);
}

const shell = detectShell();

if (!shell) {
	console.error(
		`Could not detect a supported shell from $SHELL ("${process.env.SHELL ?? ""}").` +
			"\nSupported shells: bash, zsh, fish." +
			"\nYou can add completion manually — run: miso completion <shell>",
	);
	process.exit(1);
}

const configPath = getConfigPath(shell);

if (isAlreadyInstalled(configPath, shell)) {
	console.log(`Miso shell completion is already installed in ${configPath}.`);
	process.exit(0);
}

try {
	const existing = existsSync(configPath)
		? readFileSync(configPath, "utf8")
		: "";
	writeFileSync(configPath, existing + SCRIPTS[shell], "utf8");
	console.log(`✔ Miso shell completion installed to ${configPath}`);
	console.log(`  Restart your shell or run: source ${configPath}`);
} catch (err) {
	console.error(`Failed to write to ${configPath}: ${err.message}`);
	console.error(
		"You can add completion manually — run: miso completion " + shell,
	);
	process.exit(1);
}
