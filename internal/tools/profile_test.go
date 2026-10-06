package tools

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestNormalizeProfile(t *testing.T) {
	tests := []struct {
		input Profile
		want  Profile
	}{
		{"", DrivingProfile},
		{"driving", DrivingProfile},
		{"DRIVING", DrivingProfile},
		{" Driving ", DrivingProfile},
		{"motorcycle", MotorcycleProfile},
		{"MOTORCYCLE", MotorcycleProfile},
		{" Motorcycle ", MotorcycleProfile},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got, err := normalizeProfile(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Fatalf("unexpected profile: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeProfileInvalid(t *testing.T) {
	tests := []string{
		"bicycle",
		"walking",
		"taxi",
		"flying",
		"car",
		"motorbike",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := normalizeProfile(Profile(input)); err == nil {
				t.Fatalf("expected error for %q", input)
			}
		})
	}
}

// TestProfileEnumInToolSchemas proves the Profile enum lands in the inferred
// input schemas, including the nested optimize vehicles[].profile. Note the
// Enum values are typed Profile constants here (no JSON round trip yet).
func TestProfileEnumInToolSchemas(t *testing.T) {
	want := []any{DrivingProfile, MotorcycleProfile}

	routeSchema, err := schemaFor[RouteInput]()
	if err != nil {
		t.Fatalf("route input schema: %v", err)
	}
	if got := routeSchema.(*jsonschema.Schema).Properties["profile"].Enum; !reflect.DeepEqual(got, want) {
		t.Errorf("route profile enum = %v, want %v", got, want)
	}

	tableSchema, err := schemaFor[TableInput]()
	if err != nil {
		t.Fatalf("table input schema: %v", err)
	}
	if got := tableSchema.(*jsonschema.Schema).Properties["profile"].Enum; !reflect.DeepEqual(got, want) {
		t.Errorf("table profile enum = %v, want %v", got, want)
	}

	optimizeSchema, err := schemaFor[OptimizeInput]()
	if err != nil {
		t.Fatalf("optimize input schema: %v", err)
	}
	vehicleItems := optimizeSchema.(*jsonschema.Schema).Properties["vehicles"].Items
	if got := vehicleItems.Properties["profile"].Enum; !reflect.DeepEqual(got, want) {
		t.Errorf("optimize vehicles[].profile enum = %v, want %v", got, want)
	}
}

// TestRegisteredToolsExposeEnums is the acceptance test: it registers
// the real tools, round-trips tools/list over an in-memory MCP connection
// (exactly what an LLM client sees), and checks the profile, exclude and
// annotation enums plus the strict rejection of values outside them.
// Registration only builds a client; no request ever reaches the network.
func TestRegisteredToolsExposeEnums(t *testing.T) {
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterRouteTool(server, RouteConfig{APIKey: "test"})
	RegisterTableTool(server, TableConfig{APIKey: "test"})
	RegisterOptimizeTool(server, OptimizeConfig{APIKey: "test"})

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer session.Close()

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}

	schemas := make(map[string]map[string]any, len(result.Tools))
	for _, tool := range result.Tools {
		schemas[tool.Name] = tool.InputSchema.(map[string]any)
	}

	want := []any{"driving", "motorcycle"}
	for _, name := range []string{"route", "table"} {
		props := schemas[name]["properties"].(map[string]any)
		if got := props["profile"].(map[string]any)["enum"]; !reflect.DeepEqual(got, want) {
			t.Errorf("%s inputSchema.properties.profile.enum = %v, want %v", name, got, want)
		}
	}
	vehicles := schemas["optimize"]["properties"].(map[string]any)["vehicles"].(map[string]any)
	profile := vehicles["items"].(map[string]any)["properties"].(map[string]any)["profile"].(map[string]any)
	if got := profile["enum"]; !reflect.DeepEqual(got, want) {
		t.Errorf("optimize inputSchema.properties.vehicles.items.properties.profile.enum = %v, want %v", got, want)
	}

	wantExclude := []any{"toll", "motorway", "ferry"}
	exclude := schemas["route"]["properties"].(map[string]any)["exclude"].(map[string]any)
	if got := exclude["items"].(map[string]any)["enum"]; !reflect.DeepEqual(got, wantExclude) {
		t.Errorf("route inputSchema.properties.exclude.items.enum = %v, want %v", got, wantExclude)
	}

	wantAnnotations := []any{"duration", "distance"}
	annotations := schemas["table"]["properties"].(map[string]any)["annotations"].(map[string]any)
	if got := annotations["items"].(map[string]any)["enum"]; !reflect.DeepEqual(got, wantAnnotations) {
		t.Errorf("table inputSchema.properties.annotations.items.enum = %v, want %v", got, wantAnnotations)
	}

	// The SDK validates arguments against the enum before the handler runs,
	// so the dropped "car" synonym is rejected without any HTTP request.
	call, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "route",
		Arguments: map[string]any{
			"origin":      "103.8198,1.3521",
			"destination": "103.9915,1.3644",
			"profile":     "car",
		},
	})
	if err != nil {
		t.Fatalf("route call: %v", err)
	}
	if !call.IsError {
		t.Error(`expected profile "car" to be rejected by schema validation`)
	}

	// An unsupported road class is likewise rejected by the exclude items
	// enum before the handler runs.
	call, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "route",
		Arguments: map[string]any{
			"origin":      "103.8198,1.3521",
			"destination": "103.9915,1.3644",
			"exclude":     []any{"unpaved"},
		},
	})
	if err != nil {
		t.Fatalf("route call: %v", err)
	}
	if !call.IsError {
		t.Error(`expected exclude "unpaved" to be rejected by schema validation`)
	}

	// And an unsupported annotation by the table annotations items enum.
	call, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "table",
		Arguments: map[string]any{
			"coordinates": []any{"103.8198,1.3521", "103.9915,1.3644"},
			"annotations": []any{"speed"},
		},
	})
	if err != nil {
		t.Fatalf("table call: %v", err)
	}
	if !call.IsError {
		t.Error(`expected annotation "speed" to be rejected by schema validation`)
	}
}
