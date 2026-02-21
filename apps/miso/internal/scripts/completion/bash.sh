# Miso shell completion for bash
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
