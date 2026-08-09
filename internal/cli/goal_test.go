package cli

import "testing"

func TestCanonicalGoalDate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		value string
		valid bool
	}{
		{name: "leap day", value: "2028-02-29", valid: true},
		{name: "impossible date", value: "2026-02-30", valid: false},
		{name: "timestamp", value: "2026-08-09T00:00:00Z", valid: false},
		{name: "unpadded", value: "2026-8-9", valid: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := canonicalGoalDate(test.value)
			if test.valid && (err != nil || result != test.value) {
				t.Fatalf("canonicalGoalDate(%q) = %q, %v", test.value, result, err)
			}
			if !test.valid && err == nil {
				t.Fatalf("canonicalGoalDate(%q) unexpectedly succeeded", test.value)
			}
		})
	}
}
