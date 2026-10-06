package tools

import (
	"fmt"
	"strings"
)

// Annotation is one matrix the table tool may compute. It is a strict enum:
// the table input schema exposes exactly the two values below as the
// annotations list's items, and the SDK rejects any other value before the
// handler runs.
type Annotation string

const (
	// DurationAnnotation requests the travel-duration matrix.
	DurationAnnotation Annotation = "duration"
	// DistanceAnnotation requests the travel-distance matrix.
	DistanceAnnotation Annotation = "distance"
)

// normalizeAnnotations validates and deduplicates matrix annotations.
// Empty input returns nil so the client applies its default (both).
func normalizeAnnotations(annotations []Annotation) ([]Annotation, error) {
	if len(annotations) == 0 {
		return nil, nil
	}
	out := make([]Annotation, 0, len(annotations))
	seen := make(map[Annotation]bool, len(annotations))
	for _, a := range annotations {
		a = Annotation(strings.ToLower(strings.TrimSpace(string(a))))
		switch a {
		case DurationAnnotation, DistanceAnnotation:
		default:
			return nil, fmt.Errorf(
				"invalid annotation %q: must be \"duration\" or \"distance\"",
				a,
			)
		}
		if !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	return out, nil
}

// annotationStrings converts the normalized annotations to the plain strings
// the go-client request takes. A nil slice stays nil so the client applies
// its default of computing both matrices.
func annotationStrings(annotations []Annotation) []string {
	if annotations == nil {
		return nil
	}
	out := make([]string, len(annotations))
	for i, a := range annotations {
		out[i] = string(a)
	}
	return out
}
