package tools

import (
	"testing"

	justrouting "github.com/justrouting/go-client"
)

func TestParsePoint(t *testing.T) {
	point, err := parsePoint("103.8198,1.3521")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if point.Lon() != 103.8198 {
		t.Fatalf("unexpected longitude: %v", point.Lon())
	}

	if point.Lat() != 1.3521 {
		t.Fatalf("unexpected latitude: %v", point.Lat())
	}
}

func TestParsePointInvalid(t *testing.T) {
	tests := []string{
		"",
		"103.8198",
		"103.8198,1.3521,10",
		"abc,1.3521",
		"103.8198,abc",
		"181,1.3521",
		"103.8198,91",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := parsePoint(input); err == nil {
				t.Fatalf("expected error for %q", input)
			}
		})
	}
}

var _ justrouting.Point
