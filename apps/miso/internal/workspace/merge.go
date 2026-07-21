package workspace

import "github.com/ekkolyth/miso/internal/config"

// overlays a member's miso.json onto root for non-env fields.
// Env is resolved per-target elsewhere; member Repo is ignored (members are leaves).
func EffectiveConfig(root config.Config, member Member) config.Config {
	if member.ConfigPath == "" {
		// tasks are member-owned: a member with no miso.json must not inherit
		// root's task list, else a root concurrent broadcasts to every fan-out member
		effective := root
		effective.Tasks = nil
		return effective
	}
	memberCfg, err := config.Load(member.Dir)
	if err != nil {
		// same reasoning as the no-config branch above: an unreadable member
		// config must not leak root's task list into this member's fan-out
		effective := root
		effective.Tasks = nil
		return effective
	}

	merged := root
	if memberCfg.Scripts != "" {
		merged.Scripts = memberCfg.Scripts
	}
	if memberCfg.Shell != "" {
		merged.Shell = memberCfg.Shell
	}
	if len(memberCfg.Flags) > 0 {
		merged.Flags = memberCfg.Flags
	}
	if memberCfg.TuiMode != "" && memberCfg.TuiMode != "off" {
		merged.TuiMode = memberCfg.TuiMode
		merged.TuiCleanExit = memberCfg.TuiCleanExit
	}
	// tasks are member-owned: a member with none must not inherit root's task
	// list, else a root concurrent broadcasts to every fan-out member
	merged.Tasks = memberCfg.Tasks
	return merged
}
