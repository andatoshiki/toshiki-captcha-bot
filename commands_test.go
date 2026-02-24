package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestHelpText(t *testing.T) {
	t.Parallel()

	got := helpText()

	required := []string{
		"This bot protects group joins with an emoji captcha",
		"/help",
		"/version",
		"/ping",
		"/testcaptcha",
		projectURL,
		authorInfo,
	}

	for _, entry := range required {
		if !strings.Contains(got, entry) {
			t.Fatalf("helpText missing %q", entry)
		}
	}
}

func TestPublicBotCommands(t *testing.T) {
	t.Parallel()

	cmds := publicBotCommands()
	if len(cmds) != 2 {
		t.Fatalf("public command count = %d, want 2", len(cmds))
	}

	got := []string{cmds[0].Text, cmds[1].Text}
	want := []string{"help", "version"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("public commands = %v, want %v", got, want)
	}
}

func TestAdminBotCommands(t *testing.T) {
	t.Parallel()

	cmds := adminBotCommands()
	if len(cmds) != 4 {
		t.Fatalf("admin command count = %d, want 4", len(cmds))
	}

	got := []string{cmds[0].Text, cmds[1].Text, cmds[2].Text, cmds[3].Text}
	want := []string{"help", "version", "ping", "testcaptcha"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("admin commands = %v, want %v", got, want)
	}
}
