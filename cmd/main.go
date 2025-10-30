package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ekkolyth/miso/internal/cli"
	"github.com/ekkolyth/miso/internal/managers"
)

func main() {
	args := os.Args[1:]
	parsed, err := cli.ParseCLI(args)
	if err != nil {
		fail(err, true)
	}

	managerName, err := cli.DetectManager(".")
	if err != nil {
		fail(err, false)
	}

	var driver cli.Manager
	switch managerName {
	case "bun":
		driver = managers.Bun{}
	case "npm":
		driver = managers.Npm{}
	case "pnpm":
		driver = managers.Pnpm{}
	case "yarn":
		driver = managers.Yarn{}
	default:
		fail(fmt.Errorf("unsupported manager: %s", managerName), false)
	}

	var spec cli.ExecSpec
	switch parsed.Action {
	case cli.ActionInstall:
		spec = driver.BuildInstall()
	case cli.ActionAdd:
		spec = driver.BuildAdd(parsed.PackageNames)
	case cli.ActionRemove:
		spec = driver.BuildRemove(parsed.PackageNames)
	case cli.ActionRun:
		spec = driver.BuildRun(parsed.ScriptName, parsed.ScriptArgs)
	case cli.ActionDev:
		spec = driver.BuildRun("dev", parsed.ScriptArgs)
	default:
		fail(fmt.Errorf("unknown action"), true)
	}

	if err := cli.Exec(spec, managerName); err != nil {
		fail(err, false)
	}
}

func fail(err error, showUsage bool) {
	fmt.Fprintln(os.Stderr, "miso:", err)
	if showUsage {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, usageText())
	}
	os.Exit(1)
}

func usageText() string {
	return strings.TrimSpace(`
Miso – a tiny CLI for JS package managers

Usage:
  miso install
  miso add <pkg> 
  miso remove <pkg> 
  miso run <script> <args>
  miso dev <args>

      yarn.lock            -> yarn
  - Proxies only shared commands.
`)
}



