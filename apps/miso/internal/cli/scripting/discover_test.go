package scripting

import (
	"testing"

	"github.com/ekkolyth/miso/internal/testutil"
)

func TestCheckConflicts_NoConflict(t *testing.T) {
	scripts := map[string][]ScriptInfo{
		"build": {{RelativePath: "build.sh"}},
	}
	testutil.NoError(t, CheckConflicts(scripts))
}

func TestCheckConflicts_Conflict(t *testing.T) {
	scripts := map[string][]ScriptInfo{
		"build": {{RelativePath: "build.sh"}, {RelativePath: "build.js"}},
	}
	testutil.ErrorContains(t, CheckConflicts(scripts), "multiple scripts")
}
