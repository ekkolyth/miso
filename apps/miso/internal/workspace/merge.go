package workspace

import "github.com/ekkolyth/miso/internal/config"

// EffectiveConfig overlays a member's miso.json onto root for non-env fields.
// Env is resolved per-target elsewhere; member Repo is ignored (members are leaves).
func EffectiveConfig(root config.Config, m Member) config.Config {
	if m.ConfigPath == "" {
		return root
	}
	member, err := config.Load(m.Dir)
	if err != nil {
		return root
	}

	merged := root
	if member.Scripts != "" {
		merged.Scripts = member.Scripts
	}
	if member.Shell != "" {
		merged.Shell = member.Shell
	}
	if len(member.Flags) > 0 {
		merged.Flags = member.Flags
	}
	if member.TuiMode != "" && member.TuiMode != "off" {
		merged.TuiMode = member.TuiMode
		merged.TuiCleanExit = member.TuiCleanExit
	}
	if len(member.Tasks) > 0 {
		merged.Tasks = member.Tasks
	}
	return merged
}
