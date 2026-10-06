package tools

import (
	"fmt"
	"strings"
)

// Exclude is one road class the route may avoid. It is a strict enum: the
// route input schema exposes exactly the three values below as the exclude
// list's items, and the SDK rejects any other value before the handler runs.
type Exclude string

const (
	// ExcludeToll avoids toll roads.
	ExcludeToll Exclude = "toll"
	// ExcludeMotorway avoids motorways.
	ExcludeMotorway Exclude = "motorway"
	// ExcludeFerry avoids ferry crossings.
	ExcludeFerry Exclude = "ferry"
)

// normalizeExclude validates and normalizes the road classes to avoid.
// Supported values are the standard OSRM car-profile classes: "toll",
// "motorway" and "ferry". Each value is trimmed and lowercased; duplicates
// are dropped. Unknown values are rejected so unsupported classes fail fast
// with a clear error instead of reaching the engine. An empty input returns
// nil, which the client omits from the request.
func normalizeExclude(exclude []Exclude) ([]Exclude, error) {
	var out []Exclude
	seen := make(map[Exclude]bool, len(exclude))
	for _, class := range exclude {
		class = Exclude(strings.ToLower(strings.TrimSpace(string(class))))
		switch class {
		case ExcludeToll, ExcludeMotorway, ExcludeFerry:
		default:
			return nil, fmt.Errorf(
				"unsupported road class %q: must be one of \"toll\", \"motorway\", \"ferry\"",
				class,
			)
		}
		if !seen[class] {
			seen[class] = true
			out = append(out, class)
		}
	}
	return out, nil
}

// excludeStrings converts the normalized classes to the plain strings the
// go-client request takes. A nil slice stays nil so the field keeps its
// omitempty behavior.
func excludeStrings(exclude []Exclude) []string {
	if exclude == nil {
		return nil
	}
	out := make([]string, len(exclude))
	for i, class := range exclude {
		out[i] = string(class)
	}
	return out
}
