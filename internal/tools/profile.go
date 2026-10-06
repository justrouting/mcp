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
	// ProfileDriving is the default driving profile, used when the profile
	// input is omitted.
	ProfileDriving Profile = "driving"
	// ProfileMotorcycle is the motorcycle profile.
	ProfileMotorcycle Profile = "motorcycle"
)

// profileSchemaOptions make jsonschema-go render Profile fields as a JSON
// schema string enum rather than a plain string, so LLM clients see the two
// allowed values. TypeSchemas propagate into nested struct fields, which is
// how OptimizeVehicleInput.Profile (inside OptimizeInput.Vehicles) gets the
// same enum.
var profileSchemaOptions = &jsonschema.ForOptions{
	TypeSchemas: map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[Profile](): {
			Type: "string",
			Enum: []any{ProfileDriving, ProfileMotorcycle},
		},
	},
}

// schemaFor derives the JSON schema for T with the Profile enum wired in.
func schemaFor[T any]() (any, error) {
	schema, err := jsonschema.For[T](profileSchemaOptions)
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
	case "", ProfileDriving:
		return ProfileDriving, nil
	case ProfileMotorcycle:
		return ProfileMotorcycle, nil
	default:
		return "", fmt.Errorf(
			"unsupported profile %q: must be one of \"driving\", \"motorcycle\"",
			profile,
		)
	}
}
