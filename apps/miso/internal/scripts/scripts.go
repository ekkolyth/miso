package scripts

import _ "embed"

//go:embed completion/bash.sh
var Bash string

//go:embed completion/zsh.sh
var Zsh string

//go:embed completion/fish.sh
var Fish string
