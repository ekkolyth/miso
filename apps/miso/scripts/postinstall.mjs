#!/usr/bin/env node

/**
 * Postinstall: automatically install shell completions.
 * Runs on npm install (global or local) and when using the local install script.
 */

import { readFileSync, writeFileSync, existsSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

if (!process.env.HOME) process.exit(0);

const SCRIPTS = {
  bash: `_miso_completion() {
  local cur prev
  cur="${"$"}{COMP_WORDS[COMP_CWORD]}"
  if [ $COMP_CWORD -ge 1 ]; then
    prev="${"$"}{COMP_WORDS[COMP_CWORD-1]}"
  else
    prev="miso"
  fi
  COMPREPLY=($(miso __complete miso "$cur" "$prev" 2>/dev/null))
}

complete -F _miso_completion miso
`,
  zsh: `_miso() {
  local cur="${"$"}{words[CURRENT]}"
  local prev="${"$"}{words[CURRENT-1]}"
  local -a completions
  completions=("${"$"}{(f)$(miso __complete miso "$cur" "$prev" 2>/dev/null)}")
  compadd -a completions
}

compdef _miso miso
`,
  fish: `function __miso_complete
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

function getShell() {
  const shell = process.env.SHELL || '';
  const base = shell.split('/').pop() || '';
  if (base.startsWith('bash')) return 'bash';
  if (base.startsWith('zsh')) return 'zsh';
  if (base.startsWith('fish')) return 'fish';
  return null;
}

function getConfigPath(shell) {
  const home = process.env.HOME;
  switch (shell) {
    case 'bash': return join(home, '.bashrc');
    case 'zsh': return join(home, '.zshrc');
    case 'fish': return join(home, '.config', 'fish', 'config.fish');
    default: return null;
  }
}

function isInstalled(configPath, shell) {
  if (!existsSync(configPath)) return false;
  const content = readFileSync(configPath, 'utf8');
  if (shell === 'bash') return content.includes('_miso_completion');
  if (shell === 'zsh') return content.includes('compdef _miso miso');
  if (shell === 'fish') return content.includes('complete -c miso');
  return false;
}

const shell = getShell();
if (!shell) process.exit(0);

const configPath = getConfigPath(shell);
if (!configPath) process.exit(0);

if (isInstalled(configPath, shell)) process.exit(0);

const script = SCRIPTS[shell];
const toAppend = `\n# Miso shell completion\n${script}`;

try {
  const content = existsSync(configPath) ? readFileSync(configPath, 'utf8') : '';
  writeFileSync(configPath, content + toAppend, 'utf8');
} catch {
  process.exit(0);
}
