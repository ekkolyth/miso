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

// ResolveTarget classifies a bare token: member, then configured task, else script.
// "global" is reserved and never resolves.
func ResolveTarget(token string, members []Member, root string, cfg config.Config) (Target, error) {
	if token == "global" {
		return Target{}, fmt.Errorf("%q is reserved and cannot be a target", token)
	}

	var matched []Member
	for _, member := range members {
		if matchesMember(token, member, root) {
			matched = append(matched, member)
		}
	}
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

// FindWorkspace resolves a member by basename, relative path, or package.json name.
func FindWorkspace(name string, members []Member, root string) (Member, error) {
	var matched []Member
	for _, member := range members {
		if matchesMember(name, member, root) {
			matched = append(matched, member)
		}
	}
	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return Member{}, fmt.Errorf("workspace %q not found (available: %s)", name, memberNames(members))
	default:
		return Member{}, fmt.Errorf("workspace %q is ambiguous — matches multiple members", name)
	}
}

// WorkspaceFromCWD returns the member whose directory contains cwd.
func WorkspaceFromCWD(cwd string, members []Member) (Member, bool) {
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

func matchesMember(name string, member Member, root string) bool {
	if filepath.Base(member.Dir) == name {
		return true
	}
	if rel, err := filepath.Rel(root, member.Dir); err == nil &&
		filepath.ToSlash(rel) == filepath.ToSlash(name) {
		return true
	}
	return member.Name != "" && member.Name == name
}

func memberNames(members []Member) string {
	names := make([]string, 0, len(members))
	for _, member := range members {
		names = append(names, filepath.Base(member.Dir))
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
