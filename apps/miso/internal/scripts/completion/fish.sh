# Miso shell completion for fish
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
