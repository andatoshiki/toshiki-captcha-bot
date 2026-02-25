package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tele "gopkg.in/telebot.v3"
)

const commandScopeStateFileSuffix = ".command-scopes.json"

type commandScopeState struct {
	Scopes []tele.CommandScope `json:"scopes"`
}

func commandScopeStatePathForConfig(configPath string) string {
	path := strings.TrimSpace(configPath)
	if path == "" {
		path = defaultConfigPath
	}

	clean := filepath.Clean(path)
	base := filepath.Base(clean)
	dir := filepath.Dir(clean)

	stateFile := fmt.Sprintf(".%s%s", base, commandScopeStateFileSuffix)
	return filepath.Join(dir, stateFile)
}

func normalizeCommandScope(scope tele.CommandScope) tele.CommandScope {
	return tele.CommandScope{
		Type:   scope.Type,
		ChatID: scope.ChatID,
		UserID: scope.UserID,
	}
}

func commandScopeKey(scope tele.CommandScope) string {
	normalized := normalizeCommandScope(scope)
	return fmt.Sprintf("%s:%d:%d", normalized.Type, normalized.ChatID, normalized.UserID)
}

func uniqueSortedCommandScopes(scopes []tele.CommandScope) []tele.CommandScope {
	if len(scopes) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(scopes))
	uniq := make([]tele.CommandScope, 0, len(scopes))
	for _, scope := range scopes {
		scope = normalizeCommandScope(scope)
		key := commandScopeKey(scope)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		uniq = append(uniq, scope)
	}

	sort.Slice(uniq, func(i, j int) bool {
		if uniq[i].Type != uniq[j].Type {
			return uniq[i].Type < uniq[j].Type
		}
		if uniq[i].ChatID != uniq[j].ChatID {
			return uniq[i].ChatID < uniq[j].ChatID
		}
		return uniq[i].UserID < uniq[j].UserID
	})

	return uniq
}

func mergeCommandScopes(scopeSets ...[]tele.CommandScope) []tele.CommandScope {
	merged := make([]tele.CommandScope, 0)
	for _, set := range scopeSets {
		merged = append(merged, set...)
	}
	return uniqueSortedCommandScopes(merged)
}

func diffCommandScopes(current, desired []tele.CommandScope) []tele.CommandScope {
	if len(current) == 0 {
		return nil
	}

	desiredSet := make(map[string]struct{}, len(desired))
	for _, scope := range desired {
		desiredSet[commandScopeKey(scope)] = struct{}{}
	}

	stale := make([]tele.CommandScope, 0)
	for _, scope := range current {
		scope = normalizeCommandScope(scope)
		if _, ok := desiredSet[commandScopeKey(scope)]; ok {
			continue
		}
		stale = append(stale, scope)
	}

	return uniqueSortedCommandScopes(stale)
}

func loadCommandScopeState(path string) ([]tele.CommandScope, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read command scope state file %q: %w", path, err)
	}

	state := commandScopeState{}
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("decode command scope state file %q: %w", path, err)
	}

	return uniqueSortedCommandScopes(state.Scopes), nil
}

func saveCommandScopeState(path string, scopes []tele.CommandScope) error {
	state := commandScopeState{
		Scopes: uniqueSortedCommandScopes(scopes),
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode command scope state file %q: %w", path, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create command scope state directory for %q: %w", path, err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return fmt.Errorf("write command scope state file %q: %w", path, err)
	}

	return nil
}
