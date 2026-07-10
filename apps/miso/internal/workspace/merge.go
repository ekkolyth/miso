package workspace

import "github.com/ekkolyth/miso/internal/config"

// overlays a member's miso.json onto root for non-env fields.
// Env is resolved per-target elsewhere; member Repo is ignored (members are leaves).
func EffectiveConfig(root config.Config, member Member) config.Config {
	if member.ConfigPath == "" {
		return root
	}
	memberCfg, err := config.Load(member.Dir)
	if err != nil {
		return root
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
	if len(memberCfg.Tasks) > 0 {
		merged.Tasks = memberCfg.Tasks
	}
	return merged
}
