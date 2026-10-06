package tools

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

// Profile selects the routing profile for the route, table and optimize
// tools. It is a strict enum: the MCP input schemas expose exactly the two
// values below, and the SDK rejects any other value before the handler runs.
type Profile string

const (
	// DrivingProfile is the default driving profile, used when the profile
	// input is omitted.
	DrivingProfile Profile = "driving"
	// MotorcycleProfile is the motorcycle profile.
	MotorcycleProfile Profile = "motorcycle"
)

// toolSchemaOptions make jsonschema-go render the enum types as JSON schema
// string enums rather than plain strings, so LLM clients see the allowed
// values. TypeSchemas propagate into nested struct fields, which is how
// OptimizeVehicleInput.Profile (inside OptimizeInput.Vehicles) and the
// exclude list's items get their enums.
var toolSchemaOptions = &jsonschema.ForOptions{
	TypeSchemas: map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[Profile](): {
			Type: "string",
			Enum: []any{DrivingProfile, MotorcycleProfile},
		},
		reflect.TypeFor[Exclude](): {
			Type: "string",
			Enum: []any{ExcludeToll, ExcludeMotorway, ExcludeFerry},
		},
	},
}

// schemaFor derives the JSON schema for T with the enum types wired in.
func schemaFor[T any]() (any, error) {
	schema, err := jsonschema.For[T](toolSchemaOptions)
	if err != nil {
		return nil, fmt.Errorf("derive JSON schema: %w", err)
	}
	return schema, nil
}

// mustSchema is schemaFor, panicking on error. Tool registration cannot
// return errors, and a failure here is a programming error in static struct
// definitions, so panicking matches mcp.AddTool's own behavior.
func mustSchema[T any](what string) any {
	schema, err := schemaFor[T]()
	if err != nil {
		panic(fmt.Errorf("%s: %w", what, err))
	}
	return schema
}

// normalizeProfile maps the input Profile onto the two supported routing
// profiles, trimming and case-folding like before. An empty profile (field
// omitted) selects the driving default. Synonyms such as "car" and
// "motorbike" are no longer accepted; any other value is rejected so
// unsupported profiles fail fast with a clear error instead of silently
// returning a driving route.
func normalizeProfile(profile Profile) (Profile, error) {
	switch Profile(strings.ToLower(strings.TrimSpace(string(profile)))) {
	case "", DrivingProfile:
		return DrivingProfile, nil
	case MotorcycleProfile:
		return MotorcycleProfile, nil
	default:
		return "", fmt.Errorf(
			"unsupported profile %q: must be one of \"driving\", \"motorcycle\"",
			profile,
		)
	}
}
