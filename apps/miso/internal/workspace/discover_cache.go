package workspace

import (
	"sync"

	"github.com/ekkolyth/miso/internal/config"
)

// cachedDiscovery holds one root's DiscoverMembers result, computed at most
// once regardless of how many call sites ask for it.
type cachedDiscovery struct {
	once    sync.Once
	members []Member
	err     error
}

// discoveryCache is keyed by root — the only input DiscoverMembers actually
// reads (cfg is unused) — and is safe across the process: the workspace
// layout can't change mid-invocation, and a real miso run only ever asks
// about the one root it started from.
var discoveryCache sync.Map // root string -> *cachedDiscovery

// DiscoverMembersCached wraps DiscoverMembers with a per-root memo. The CWD
// scoping check, the root-scope check, scope-filter resolution, and per-script
// discovery all ask for the same root's members within one invocation, so
// repeated lookups reuse the first glob instead of re-walking the tree.
func DiscoverMembersCached(root string, cfg config.Config) ([]Member, error) {
	v, _ := discoveryCache.LoadOrStore(root, &cachedDiscovery{})
	entry := v.(*cachedDiscovery)
	entry.once.Do(func() {
		entry.members, entry.err = DiscoverMembers(root, cfg)
	})
	return entry.members, entry.err
}
