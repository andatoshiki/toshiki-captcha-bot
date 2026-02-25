package app

import (
	"strings"
	"testing"
)

func TestVersionTextMarkdown(t *testing.T) {
	t.Parallel()

	oldVersion := Version
	oldCommit := Commit
	oldBuildTime := BuildTime
	Version = "1.1.0"
	Commit = "da42e656c0d47c905c552b2380a5519de20d9560"
	BuildTime = "2026-02-24T05:00:26Z"
	t.Cleanup(func() {
		Version = oldVersion
		Commit = oldCommit
		BuildTime = oldBuildTime
	})

	out := versionTextMarkdown()
	required := []string{
		"Version: `1.1.0`",
		"Commit: `da42e656c0d47c905c552b2380a5519de20d9560`",
		"Build time: `2026-02-24T05:00:26Z`",
		"Go: `",
	}
	for _, entry := range required {
		if !strings.Contains(out, entry) {
			t.Fatalf("versionTextMarkdown missing %q in output:\n%s", entry, out)
		}
	}
}

func TestInlineCodeEscapesBackticks(t *testing.T) {
	t.Parallel()

	got := inlineCode("a`b")
	want := "`a\\`b`"
	if got != want {
		t.Fatalf("inlineCode = %q, want %q", got, want)
	}
}
