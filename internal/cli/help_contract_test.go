package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONHelpExposesMachineReadableContract(t *testing.T) {
	var output bytes.Buffer
	command := New("test", strings.NewReader(""), &output, &output)
	command.SetArgs([]string{"record", "update", "--help", "--format", "json"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var contract helpContract
	if err := json.Unmarshal(output.Bytes(), &contract); err != nil {
		t.Fatal(err)
	}
	if contract.SchemaVersion != "1" || contract.Command != "sateia record update" || contract.Risk != riskWrite {
		t.Fatalf("unexpected contract header: %#v", contract)
	}
	flags := make(map[string]helpFlag, len(contract.Flags))
	for _, flag := range contract.Flags {
		flags[flag.Name] = flag
	}
	if !flags["record-id"].Required || flags["expected-version"].Type != "int64" {
		t.Fatalf("missing required field metadata: %#v", flags)
	}
	if !strings.Contains(flags["note"].Description, "food name and quantity") {
		t.Fatalf("missing note guidance: %#v", flags["note"])
	}
	if !hasHelpRule(contract.Rules, "ALL_OR_NONE", "carbohydrate", "energy", "fat", "protein") {
		t.Fatalf("missing nutrient rule: %#v", contract.Rules)
	}
	if !hasHelpRule(contract.Rules, "MUTUALLY_EXCLUSIVE", "clear-note", "note") {
		t.Fatalf("missing note rule: %#v", contract.Rules)
	}
}

func TestHelpFormatRejectsInvalidAndNonHelpUse(t *testing.T) {
	for _, args := range [][]string{
		{"record", "list", "--help", "--format", "yaml"},
		{"environment", "--format", "json"},
	} {
		command := New("test", strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{})
		command.SetArgs(args)
		if err := command.Execute(); err == nil {
			t.Fatalf("command unexpectedly accepted %v", args)
		}
	}
}

func hasHelpRule(rules []helpRule, kind string, fields ...string) bool {
	wanted := strings.Join(fields, ",")
	for _, rule := range rules {
		if rule.Kind == kind && strings.Join(rule.Fields, ",") == wanted {
			return true
		}
	}
	return false
}
