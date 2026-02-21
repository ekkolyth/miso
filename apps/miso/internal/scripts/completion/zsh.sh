# Miso shell completion for zsh
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
