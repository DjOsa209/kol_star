package main

import "testing"

func TestSelectedSyncPlatforms(t *testing.T) {
	tests := []struct {
		name      string
		body      map[string]any
		wantAll   bool
		wantCount int
		wantError bool
	}{
		{name: "missing means all", body: map[string]any{}, wantAll: true},
		{name: "empty means all", body: map[string]any{"platforms": []any{}}, wantAll: true},
		{name: "normalizes aliases", body: map[string]any{"platforms": []any{"youtube", "ins"}}, wantCount: 2},
		{name: "rejects unsupported platform", body: map[string]any{"platforms": []any{"Facebook"}}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := selectedSyncPlatforms(test.body)
			if (err != nil) != test.wantError {
				t.Fatalf("selectedSyncPlatforms() error = %v, wantError %v", err, test.wantError)
			}
			if test.wantAll && len(got) != 0 {
				t.Fatalf("selectedSyncPlatforms() = %v, want all platforms", got)
			}
			if len(got) != test.wantCount {
				t.Fatalf("selectedSyncPlatforms() count = %d, want %d", len(got), test.wantCount)
			}
		})
	}
}

func TestOptionalConfigValue(t *testing.T) {
	tests := []struct {
		value any
		want  string
	}{
		{value: nil, want: ""},
		{value: "<nil>", want: ""},
		{value: " http://127.0.0.1:7890 ", want: "http://127.0.0.1:7890"},
	}
	for _, test := range tests {
		if got := optionalConfigValue(test.value); got != test.want {
			t.Fatalf("optionalConfigValue(%v) = %q, want %q", test.value, got, test.want)
		}
	}
}
