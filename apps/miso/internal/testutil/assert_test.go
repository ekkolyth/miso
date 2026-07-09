package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEqual_Mismatch_Reports(t *testing.T) {
	mock := &testing.T{}
	Equal(mock, 1, 2)
	if !mock.Failed() {
		t.Fatal("Equal should have reported a failure for 1 != 2")
	}
}

func TestEqual_Match_Silent(t *testing.T) {
	mock := &testing.T{}
	Equal(mock, "a", "a")
	if mock.Failed() {
		t.Fatal("Equal should not fail for equal values")
	}
}

func TestErrorContains_Missing_Reports(t *testing.T) {
	mock := &testing.T{}
	ErrorContains(mock, nil, "boom")
	if !mock.Failed() {
		t.Fatal("ErrorContains should fail when err is nil")
	}
}

func TestWriteFiles_CreatesFiles(t *testing.T) {
	dir := t.TempDir()
	WriteFiles(t, dir, "bun.lockb", "package.json")
	for _, name := range []string{"bun.lockb", "package.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}
}
