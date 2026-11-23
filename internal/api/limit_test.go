package api

import (
	"errors"
	"testing"
)

func TestParseLimit(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{name: "absent uses default", raw: "", want: 50},
		{name: "smallest valid", raw: "1", want: 1},
		{name: "normal value", raw: "20", want: 20},
		{name: "exactly the max", raw: "200", want: 200},
		{name: "just over the max is clamped", raw: "201", want: 200},
		{name: "far over the max is clamped", raw: "500", want: 200},
		{name: "zero", raw: "0", wantErr: true},
		{name: "negative", raw: "-5", wantErr: true},
		{name: "not a number", raw: "abc", wantErr: true},
		{name: "decimal", raw: "1.5", wantErr: true},
		{name: "trailing text", raw: "20abc", wantErr: true},
		{name: "leading space", raw: " 20", wantErr: true},
		{name: "too big for an int", raw: "99999999999999999999", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLimit(tt.raw)

			if tt.wantErr {
				if !errors.Is(err, errInvalidLimit) {
					t.Fatalf("parseLimit(%q): got error %v, want %v", tt.raw, err, errInvalidLimit)
				}
				if got != 0 {
					t.Errorf("parseLimit(%q): got limit %d with an error, want 0", tt.raw, got)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseLimit(%q): unexpected error %v", tt.raw, err)
			}
			if got != tt.want {
				t.Errorf("parseLimit(%q): got %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}
