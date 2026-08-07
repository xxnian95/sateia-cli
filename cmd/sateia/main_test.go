package main

import "testing"

func TestResolvedVersionPrefersModuleVersionForTaggedInstall(t *testing.T) {
	if got := resolvedVersion("dev", "v1.0.0"); got != "v1.0.0" {
		t.Fatalf("resolved version = %q", got)
	}
	if got := resolvedVersion("v2.0.0", "v1.0.0"); got != "v2.0.0" {
		t.Fatalf("linked version = %q", got)
	}
	if got := resolvedVersion("dev", "(devel)"); got != "dev" {
		t.Fatalf("development version = %q", got)
	}
	if got := resolvedVersion("dev", "v0.0.0-20260807103817-4b70e798ce5b+dirty"); got != "dev" {
		t.Fatalf("pseudo version = %q", got)
	}
}
