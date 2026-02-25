package app

import (
	"path/filepath"
	"reflect"
	"testing"

	tele "gopkg.in/telebot.v3"
)

func TestCommandScopeStatePathForConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "default when empty input",
			input:  "",
			expect: filepath.Join(".", ".config.yaml.command-scopes.json"),
		},
		{
			name:   "relative config path",
			input:  "configs/dev.yaml",
			expect: filepath.Join("configs", ".dev.yaml.command-scopes.json"),
		},
		{
			name:   "absolute config path",
			input:  "/tmp/captcha/config.yaml",
			expect: "/tmp/captcha/.config.yaml.command-scopes.json",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := commandScopeStatePathForConfig(tt.input)
			if got != tt.expect {
				t.Fatalf("commandScopeStatePathForConfig() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestDiffCommandScopes_RemovedAdminScopesBecomeStale(t *testing.T) {
	t.Parallel()

	previous := []tele.CommandScope{
		{Type: tele.CommandScopeChat, ChatID: 1001},
		{Type: tele.CommandScopeChatMember, ChatID: -1001, UserID: 1001},
		{Type: tele.CommandScopeChat, ChatID: 2002},
		{Type: tele.CommandScopeChatMember, ChatID: -1001, UserID: 2002},
	}
	desired := []tele.CommandScope{
		{Type: tele.CommandScopeChat, ChatID: 1001},
		{Type: tele.CommandScopeChatMember, ChatID: -1001, UserID: 1001},
	}

	got := diffCommandScopes(previous, desired)
	want := []tele.CommandScope{
		{Type: tele.CommandScopeChat, ChatID: 2002},
		{Type: tele.CommandScopeChatMember, ChatID: -1001, UserID: 2002},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffCommandScopes() = %#v, want %#v", got, want)
	}
}

func TestCommandScopeStateRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, ".config.yaml.command-scopes.json")

	input := []tele.CommandScope{
		{Type: tele.CommandScopeChat, ChatID: 2002},
		{Type: tele.CommandScopeChat, ChatID: 1001},
		{Type: tele.CommandScopeChat, ChatID: 1001}, // duplicate should be deduped
		{Type: tele.CommandScopeChatMember, ChatID: -1001, UserID: 1001},
	}

	if err := saveCommandScopeState(path, input); err != nil {
		t.Fatalf("saveCommandScopeState returned error: %v", err)
	}

	got, err := loadCommandScopeState(path)
	if err != nil {
		t.Fatalf("loadCommandScopeState returned error: %v", err)
	}

	want := []tele.CommandScope{
		{Type: tele.CommandScopeChat, ChatID: 1001},
		{Type: tele.CommandScopeChat, ChatID: 2002},
		{Type: tele.CommandScopeChatMember, ChatID: -1001, UserID: 1001},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadCommandScopeState() = %#v, want %#v", got, want)
	}
}
