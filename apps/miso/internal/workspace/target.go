package workspace

import (
	"fmt"
	"path/filepath"
	"strings"

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

// ResolveScopes maps each @-prefixed scope token to exactly one member, matching
// by identity in priority order: exact package name (bare `web` or scoped
// `@ekko/web`), then a scoped package's short-name (`@ekko/web` → `web`), then
// relative path, and only as a last resort the directory basename. A higher tier
// always wins, so a workspace named `web` is never shadowed by an unrelated one
// that merely sits in a folder called `web`. Ambiguity *within* the winning
// tier, or no match at all, is an error.
func ResolveScopes(scopes []string, members []Member, root string) ([]Member, error) {
	seen := make(map[string]bool)
	var out []Member
	for _, token := range scopes {
		member, err := resolveScope(token, members, root)
		if err != nil {
			return nil, err
		}
		key := member.Name + "\x00" + member.Dir
		if !seen[key] {
			seen[key] = true
			out = append(out, member)
		}
	}
	return out, nil
}

func resolveScope(token string, members []Member, root string) (Member, error) {
	bare := strings.TrimPrefix(token, "@")
	tiers := []func(Member) bool{
		func(m Member) bool { return m.Name != "" && (m.Name == bare || m.Name == "@"+bare) }, // exact package name
		func(m Member) bool { return m.Name != "" && scopeShortName(m.Name) == bare },         // scoped package short-name
		func(m Member) bool { return relPath(m.Dir, root) == bare },                           // relative path from root
		func(m Member) bool { return filepath.Base(m.Dir) == bare },                           // dir basename (last resort)
	}
	for _, matches := range tiers {
		var hits []Member
		for _, member := range members {
			if matches(member) {
				hits = append(hits, member)
			}
		}
		switch len(hits) {
		case 1:
			return hits[0], nil
		case 0:
			continue
		default:
			return Member{}, fmt.Errorf("scope %q is ambiguous — matches %s; qualify by path (e.g. @%s)",
				token, memberPaths(hits, root), relPath(hits[0].Dir, root))
		}
	}
	return Member{}, fmt.Errorf("scope %q matched no workspace (available: %s)", token, memberNames(members))
}

// scopeShortName returns the segment after the last "/" of a scoped package name
// (@ekko/web → web); an unscoped name is returned whole.
func scopeShortName(name string) string {
	if slash := strings.LastIndex(name, "/"); slash >= 0 {
		return name[slash+1:]
	}
	return name
}

func relPath(dir, root string) string {
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(rel)
}

// memberPaths lists members by relative path, for disambiguation messages.
func memberPaths(members []Member, root string) string {
	parts := make([]string, 0, len(members))
	for _, member := range members {
		parts = append(parts, relPath(member.Dir, root))
	}
	return strings.Join(parts, ", ")
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
