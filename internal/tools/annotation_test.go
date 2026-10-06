package tools

import (
	"reflect"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func TestNormalizeAnnotations(t *testing.T) {
	tests := []struct {
		name string
		in   []Annotation
		want []Annotation
	}{
		{"nil stays nil so the client default applies", nil, nil},
		{"empty stays nil", []Annotation{}, nil},
		{"single annotation", []Annotation{"duration"}, []Annotation{"duration"}},
		{
			"case is normalized and duplicates dropped",
			[]Annotation{"Distance", " distance "},
			[]Annotation{"distance"},
		},
		{
			"order is preserved",
			[]Annotation{"duration", "distance"},
			[]Annotation{"duration", "distance"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeAnnotations(tt.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("unexpected result: %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("unexpected result: %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestNormalizeAnnotationsInvalid(t *testing.T) {
	for _, in := range [][]Annotation{{"speed"}, {"duration", "walking"}} {
		if _, err := normalizeAnnotations(in); err == nil {
			t.Fatalf("expected error for %v", in)
		}
	}
}

// TestAnnotationEnumInTableSchema proves the Annotation enum lands in the
// table input schema's annotations list items.
func TestAnnotationEnumInTableSchema(t *testing.T) {
	want := []any{DurationAnnotation, DistanceAnnotation}

	schema, err := schemaFor[TableInput]()
	if err != nil {
		t.Fatalf("table input schema: %v", err)
	}
	if got := schema.(*jsonschema.Schema).Properties["annotations"].Items.Enum; !reflect.DeepEqual(got, want) {
		t.Errorf("table annotations items enum = %v, want %v", got, want)
	}
}
