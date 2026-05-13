package codex

import (
	"reflect"
	"testing"
)

func TestSerializeConfigOverrides(t *testing.T) {
	overrides, err := serializeConfigOverrides(CodexConfigObject{
		"approval_policy": "never",
		"retry_budget":    3,
		"provider": map[string]string{
			"name": "mock",
		},
		"sandbox_workspace_write": CodexConfigObject{
			"network_access": true,
		},
		"tool_rules": CodexConfigObject{
			"allow": []string{"git status", "git diff"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{
		`approval_policy="never"`,
		`provider.name="mock"`,
		`retry_budget=3`,
		`sandbox_workspace_write.network_access=true`,
		`tool_rules.allow=["git status", "git diff"]`,
	}
	if !reflect.DeepEqual(overrides, expected) {
		t.Fatalf("overrides mismatch\nwant: %#v\n got: %#v", expected, overrides)
	}
}

func TestSerializeConfigOverridesRejectsInvalidValues(t *testing.T) {
	tests := []CodexConfigObject{
		{"bad": nil},
		{"bad": []any{1, nil}},
		{"bad": map[string]any{"": true}},
	}

	for _, test := range tests {
		if _, err := serializeConfigOverrides(test); err == nil {
			t.Fatalf("expected error for %#v", test)
		}
	}
}
