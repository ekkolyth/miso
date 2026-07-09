package workspace

import (
	"fmt"
	"path/filepath"

	"github.com/ekkolyth/miso/internal/config"
)

type TargetKind int

const (
	TargetMember TargetKind = iota
	TargetScript
	TargetTask
)

type Target struct {
	Kind TargetKind
	Name string
	Dir  string
}

// classifies a bare token: member, then configured task, else script.
// "global" is reserved and never resolves.
func ResolveTarget(token string, members []Member, root string, cfg config.Config) (Target, error) {
	if token == "global" {
		return Target{}, fmt.Errorf("%q is reserved and cannot be a target", token)
	}

	matched := resolveMembers(token, members, root)
	switch len(matched) {
	case 1:
		return Target{Kind: TargetMember, Name: matched[0].Name, Dir: matched[0].Dir}, nil
	case 0:
		// fall through to script/task
	default:
		return Target{}, fmt.Errorf("workspace %q is ambiguous — matches multiple members", token)
	}

	if _, ok := cfg.Tasks[token]; ok {
		return Target{Kind: TargetTask, Name: token}, nil
	}
	return Target{Kind: TargetScript, Name: token}, nil
}

// matches a member by package name or relative path, falling back to directory
// basename only when nothing matches the canonical identity.
func Find(name string, members []Member, root string) (Member, error) {
	matched := resolveMembers(name, members, root)
	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return Member{}, fmt.Errorf("workspace %q not found (available: %s)", name, memberNames(members))
	default:
		return Member{}, fmt.Errorf("workspace %q is ambiguous — matches multiple members", name)
	}
}

// the member whose directory contains cwd.
func FromCWD(cwd string, members []Member) (Member, bool) {
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return Member{}, false
	}
	for _, member := range members {
		absDir, err := filepath.Abs(member.Dir)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(absDir, absCwd)
		if err != nil {
			continue
		}
		if rel == "." || (len(rel) > 0 && rel[0] != '.') {
			return member, true
		}
	}
	return Member{}, false
}

// matchesExplicit reports an exact package-name or relative-path match — the
// canonical ways a workspace is identified (pnpm/bun/turbo all key off the
// package name). Directory basename is deliberately excluded here.
func matchesExplicit(name string, member Member, root string) bool {
	if member.Name != "" && member.Name == name {
		return true
	}
	rel, err := filepath.Rel(root, member.Dir)
	return err == nil && filepath.ToSlash(rel) == filepath.ToSlash(name)
}

// resolveMembers returns the members a token identifies, preferring an exact
// package-name/relpath match and only falling back to directory basename when
// no canonical match exists — so a distinctly-named workspace never collides
// with an unrelated directory that happens to share a basename.
func resolveMembers(name string, members []Member, root string) []Member {
	var explicit, byBasename []Member
	for _, member := range members {
		if matchesExplicit(name, member, root) {
			explicit = append(explicit, member)
		} else if filepath.Base(member.Dir) == name {
			byBasename = append(byBasename, member)
		}
	}
	if len(explicit) > 0 {
		return explicit
	}
	return byBasename
}

func memberNames(members []Member) string {
	names := make([]string, 0, len(members))
	for _, member := range members {
		name := member.Name
		if name == "" {
			name = filepath.Base(member.Dir)
		}
		names = append(names, name)
	}
	out := ""
	for i, name := range names {
		if i > 0 {
			out += ", "
		}
		out += name
	}
	return out
}
