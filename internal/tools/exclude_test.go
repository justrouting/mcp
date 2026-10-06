package tools

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func TestNormalizeExclude(t *testing.T) {
	tests := []struct {
		input []Exclude
		want  []Exclude
	}{
		{nil, nil},
		{[]Exclude{}, nil},
		{[]Exclude{"toll"}, []Exclude{"toll"}},
		{[]Exclude{"motorway", "ferry"}, []Exclude{"motorway", "ferry"}},
		{[]Exclude{" Toll ", "MOTORWAY"}, []Exclude{"toll", "motorway"}},
		{[]Exclude{"toll", "toll", "ferry"}, []Exclude{"toll", "ferry"}},
	}

	for _, tt := range tests {
		t.Run(strings.Join(excludeStrings(tt.input), ","), func(t *testing.T) {
			got, err := normalizeExclude(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("unexpected exclude: got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeExcludeInvalid(t *testing.T) {
	tests := [][]Exclude{
		{"unpaved"},
		{"toll", "highway"},
		{""},
	}

	for _, input := range tests {
		t.Run(strings.Join(excludeStrings(input), ","), func(t *testing.T) {
			if _, err := normalizeExclude(input); err == nil {
				t.Fatalf("expected error for %v", input)
			}
		})
	}
}

// TestExcludeEnumInRouteSchema proves the Exclude enum lands in the route
// input schema's exclude list items.
func TestExcludeEnumInRouteSchema(t *testing.T) {
	want := []any{ExcludeToll, ExcludeMotorway, ExcludeFerry}

	schema, err := schemaFor[RouteInput]()
	if err != nil {
		t.Fatalf("route input schema: %v", err)
	}
	if got := schema.(*jsonschema.Schema).Properties["exclude"].Items.Enum; !reflect.DeepEqual(got, want) {
		t.Errorf("route exclude items enum = %v, want %v", got, want)
	}
}
