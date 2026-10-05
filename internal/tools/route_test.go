package tools

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
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

func TestNormalizeProfile(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "driving"},
		{"car", "driving"},
		{"driving", "driving"},
		{"CAR", "driving"},
		{"motorcycle", "motorcycle"},
		{"motorbike", "motorcycle"},
		{" Motorcycle ", "motorcycle"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
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
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := normalizeProfile(input); err == nil {
				t.Fatalf("expected error for %q", input)
			}
		})
	}
}

func TestNormalizeExclude(t *testing.T) {
	tests := []struct {
		input []string
		want  []string
	}{
		{nil, nil},
		{[]string{}, nil},
		{[]string{"toll"}, []string{"toll"}},
		{[]string{"motorway", "ferry"}, []string{"motorway", "ferry"}},
		{[]string{" Toll ", "MOTORWAY"}, []string{"toll", "motorway"}},
		{[]string{"toll", "toll", "ferry"}, []string{"toll", "ferry"}},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.input, ","), func(t *testing.T) {
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
	tests := [][]string{
		{"unpaved"},
		{"toll", "highway"},
		{""},
	}

	for _, input := range tests {
		t.Run(strings.Join(input, ","), func(t *testing.T) {
			if _, err := normalizeExclude(input); err == nil {
				t.Fatalf("expected error for %v", input)
			}
		})
	}
}

// routeFixture mirrors a real routing response: snapped waypoints, a
// simplified polyline geometry, and one leg per consecutive waypoint pair.
// (The polyline contains a literal backtick, so it is built by concatenation.)
const routeFixture = `{
  "code": "Ok",
  "waypoints": [
    {"hint":"aaa","distance":4.216,"name":"Marina Boulevard","location":[103.81982,1.35211]},
    {"hint":"bbb","distance":8.104,"name":"Changi Coast Road","location":[103.99151,1.36442]}
  ],
  "routes": [
    {
      "geometry": "ka|` + "`" + `@_ceeEnAqB",
      "legs": [
        {
          "steps": [
            {
              "mode": "driving",
              "name": "East Coast Parkway",
              "maneuver": {"type": "depart", "location": [103.81982, 1.35211]},
              "intersections": [
                {
                  "location": [103.81982, 1.35211],
                  "bearings": [95, 275],
                  "entry": [true, false],
                  "in": 1,
                  "out": 0,
                  "classes": ["toll", "motorway"]
                }
              ]
            }
          ],
          "summary": "East Coast Parkway",
          "weight": 1583.4,
          "duration": 1583.4,
          "distance": 24512.7
        }
      ],
      "weight_name": "routability",
      "weight": 1583.4,
      "duration": 1583.4,
      "distance": 24512.7
    }
  ]
}`

// routeNoSnapFixture is an Ok response without waypoint or geometry data and
// with a leg whose road summary is empty.
const routeNoSnapFixture = `{
  "code": "Ok",
  "waypoints": [],
  "routes": [
    {"distance": 24512.7, "duration": 1583.4, "legs": [{"summary": ""}]}
  ]
}`

func TestGetRouteDecodesResponse(t *testing.T) {
	client := newTestClient(t, jsonHandler(http.StatusOK, routeFixture))

	_, output, err := getRoute(
		context.Background(),
		client,
		RouteInput{Origin: "103.8198,1.3521", Destination: "103.9915,1.3644"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.DistanceMeters != 24512.7 {
		t.Fatalf("unexpected distance_meters: %v", output.DistanceMeters)
	}
	if output.DurationSeconds != 1583.4 {
		t.Fatalf("unexpected duration_seconds: %v", output.DurationSeconds)
	}
	if output.Profile != "driving" {
		t.Fatalf("unexpected profile: %q", output.Profile)
	}
	if output.Exclude != nil {
		t.Fatalf("expected no exclude echo, got: %v", output.Exclude)
	}

	if output.Origin.Input != "103.8198,1.3521" {
		t.Fatalf("unexpected origin input: %q", output.Origin.Input)
	}
	if output.Origin.Name != "Marina Boulevard" {
		t.Fatalf("unexpected origin name: %q", output.Origin.Name)
	}
	if output.Origin.Snapped.Lon() != 103.81982 || output.Origin.Snapped.Lat() != 1.35211 {
		t.Fatalf("unexpected origin snapped: %v", output.Origin.Snapped)
	}
	if output.Origin.Distance != 4.216 {
		t.Fatalf("unexpected origin snap distance: %v", output.Origin.Distance)
	}

	if output.Destination.Input != "103.9915,1.3644" {
		t.Fatalf("unexpected destination input: %q", output.Destination.Input)
	}
	if output.Destination.Name != "Changi Coast Road" {
		t.Fatalf("unexpected destination name: %q", output.Destination.Name)
	}
	if output.Destination.Snapped.Lon() != 103.99151 || output.Destination.Snapped.Lat() != 1.36442 {
		t.Fatalf("unexpected destination snapped: %v", output.Destination.Snapped)
	}
	if output.Destination.Distance != 8.104 {
		t.Fatalf("unexpected destination snap distance: %v", output.Destination.Distance)
	}

	if output.Geometry != "ka|`@_ceeEnAqB" {
		t.Fatalf("unexpected geometry: %q", output.Geometry)
	}

	if !slices.Equal(output.Summary.MajorRoads, []string{"East Coast Parkway"}) {
		t.Fatalf("unexpected major roads: %v", output.Summary.MajorRoads)
	}
	if !output.Summary.Tolls {
		t.Fatal("expected tolls true, got false")
	}
	if output.Summary.Ferry {
		t.Fatal("expected ferry false, got true")
	}
}

func TestGetRouteQueryEncoding(t *testing.T) {
	tests := []struct {
		name        string
		input       RouteInput
		wantPath    string
		wantQuery   map[string]string
		wantAbsent  []string
		wantProfile string
		wantExclude []string
	}{
		{
			name:        "defaults request steps and the polyline simplified overview",
			input:       RouteInput{Origin: "103.8198,1.3521", Destination: "103.9915,1.3644"},
			wantPath:    "/route/v1/driving/103.8198,1.3521;103.9915,1.3644",
			wantQuery:   map[string]string{"geometries": "polyline", "overview": "simplified", "steps": "true"},
			wantAbsent:  []string{"exclude"},
			wantProfile: "driving",
		},
		{
			name:        "motorcycle profile goes into the path and echoes back",
			input:       RouteInput{Origin: "103.8198,1.3521", Destination: "103.9915,1.3644", Profile: "motorcycle"},
			wantPath:    "/route/v1/motorcycle/103.8198,1.3521;103.9915,1.3644",
			wantQuery:   map[string]string{"geometries": "polyline", "overview": "simplified", "steps": "true"},
			wantProfile: "motorcycle",
		},
		{
			name:        "car profile normalizes to driving",
			input:       RouteInput{Origin: "103.8198,1.3521", Destination: "103.9915,1.3644", Profile: "car"},
			wantPath:    "/route/v1/driving/103.8198,1.3521;103.9915,1.3644",
			wantQuery:   map[string]string{"geometries": "polyline", "overview": "simplified", "steps": "true"},
			wantProfile: "driving",
		},
		{
			name:        "excluded classes are sent comma-joined and echoed",
			input:       RouteInput{Origin: "103.8198,1.3521", Destination: "103.9915,1.3644", Exclude: []string{"toll", "motorway"}},
			wantPath:    "/route/v1/driving/103.8198,1.3521;103.9915,1.3644",
			wantQuery:   map[string]string{"geometries": "polyline", "overview": "simplified", "steps": "true", "exclude": "toll,motorway"},
			wantProfile: "driving",
			wantExclude: []string{"toll", "motorway"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			var gotQuery url.Values
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotQuery = r.URL.Query()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"code":"Ok","waypoints":[],"routes":[{"distance":1,"duration":2}]}`))
			})

			_, output, err := getRoute(
				context.Background(),
				client,
				tt.input,
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotPath != tt.wantPath {
				t.Fatalf("unexpected path: got %q, want %q", gotPath, tt.wantPath)
			}

			for key, want := range tt.wantQuery {
				if got := gotQuery.Get(key); got != want {
					t.Fatalf("unexpected %s: got %q, want %q", key, got, want)
				}
			}
			for _, key := range tt.wantAbsent {
				if got := gotQuery.Get(key); got != "" {
					t.Fatalf("expected %s to be omitted, got %q", key, got)
				}
			}

			// The normalized profile is echoed in the output.
			if output.Profile != tt.wantProfile {
				t.Fatalf("unexpected echoed profile: got %q, want %q", output.Profile, tt.wantProfile)
			}

			// The normalized exclude list is echoed in the output.
			if !slices.Equal(output.Exclude, tt.wantExclude) {
				t.Fatalf("unexpected echoed exclude: got %v, want %v", output.Exclude, tt.wantExclude)
			}
		})
	}
}

func TestGetRouteWaypointsMissing(t *testing.T) {
	client := newTestClient(t, jsonHandler(http.StatusOK, routeNoSnapFixture))

	_, output, err := getRoute(
		context.Background(),
		client,
		RouteInput{Origin: "103.8198,1.3521", Destination: "103.9915,1.3644"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.Origin.Input != "103.8198,1.3521" {
		t.Fatalf("unexpected origin input: %q", output.Origin.Input)
	}
	if len(output.Origin.Snapped) != 0 {
		t.Fatalf("expected no snapped origin, got: %v", output.Origin.Snapped)
	}
	if output.Origin.Name != "" {
		t.Fatalf("unexpected origin name: %q", output.Origin.Name)
	}
	if output.Origin.Distance != 0 {
		t.Fatalf("unexpected origin snap distance: %v", output.Origin.Distance)
	}

	if output.Destination.Input != "103.9915,1.3644" {
		t.Fatalf("unexpected destination input: %q", output.Destination.Input)
	}
	if len(output.Destination.Snapped) != 0 {
		t.Fatalf("expected no snapped destination, got: %v", output.Destination.Snapped)
	}

	if output.Geometry != "" {
		t.Fatalf("expected no geometry, got: %q", output.Geometry)
	}

	// An empty road list still serializes as [], never null.
	if output.Summary.MajorRoads == nil || len(output.Summary.MajorRoads) != 0 {
		t.Fatalf("expected empty major roads, got: %v", output.Summary.MajorRoads)
	}

	// Without step data the tolls/ferry booleans default to false.
	if output.Summary.Tolls || output.Summary.Ferry {
		t.Fatalf("expected no tolls/ferry, got tolls=%v ferry=%v", output.Summary.Tolls, output.Summary.Ferry)
	}
}

func TestGetRouteEmptyRoutes(t *testing.T) {
	client := newTestClient(t, jsonHandler(http.StatusOK, `{"code":"Ok","routes":[],"waypoints":[]}`))

	_, _, err := getRoute(
		context.Background(),
		client,
		RouteInput{Origin: "103.8198,1.3521", Destination: "103.9915,1.3644"},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no route found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetRouteEngineErrorBehindHTTP200(t *testing.T) {
	client := newTestClient(
		t,
		jsonHandler(http.StatusOK, `{"code":"NoRoute","message":"Impossible route between points"}`),
	)

	_, _, err := getRoute(
		context.Background(),
		client,
		RouteInput{Origin: "103.8198,1.3521", Destination: "103.9915,1.3644"},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	// The client turns non-Ok engine codes into *justrouting.Error; the
	// handler only wraps it.
	var apiErr *justrouting.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *justrouting.Error, got: %v", err)
	}
	if apiErr.OSRMCode != "NoRoute" {
		t.Fatalf("unexpected OSRM code: %q", apiErr.OSRMCode)
	}
}

func TestGetRouteInputValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   RouteInput
		wantErr string
	}{
		{
			name:    "invalid origin longitude",
			input:   RouteInput{Origin: "181,1.3521", Destination: "103.9915,1.3644"},
			wantErr: "invalid origin",
		},
		{
			name:    "invalid destination latitude",
			input:   RouteInput{Origin: "103.8198,1.3521", Destination: "103.8198,91"},
			wantErr: "invalid destination",
		},
		{
			name:    "unsupported profile",
			input:   RouteInput{Origin: "103.8198,1.3521", Destination: "103.9915,1.3644", Profile: "walking"},
			wantErr: "invalid profile",
		},
		{
			name:    "unsupported exclude class",
			input:   RouteInput{Origin: "103.8198,1.3521", Destination: "103.9915,1.3644", Exclude: []string{"unpaved"}},
			wantErr: "invalid exclude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("no request should reach the server")
			})

			if _, _, err := getRoute(
				context.Background(),
				client,
				tt.input,
			); err == nil {
				t.Fatalf("expected error for %+v", tt.input)
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestGetRouteAPIErrorPassthrough(t *testing.T) {
	client := newTestClient(
		t,
		jsonHandler(
			http.StatusUnauthorized,
			`{"statusCode":401,"error":"Unauthorized","message":"Invalid apiKey"}`,
		),
	)

	_, _, err := getRoute(
		context.Background(),
		client,
		RouteInput{Origin: "103.8198,1.3521", Destination: "103.9915,1.3644"},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, justrouting.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
	if !strings.Contains(err.Error(), "route calculation failed") {
		t.Fatalf("expected wrapped error, got: %v", err)
	}
}

func TestMajorRoads(t *testing.T) {
	leg := func(summary string) *justrouting.Leg {
		return &justrouting.Leg{Summary: summary}
	}

	tests := []struct {
		name string
		legs []*justrouting.Leg
		want []string
	}{
		{"no legs", nil, []string{}},
		{"consecutive duplicates collapse", []*justrouting.Leg{leg("A"), leg("A"), leg("B")}, []string{"A", "B"}},
		{"recurrence after leaving is kept", []*justrouting.Leg{leg("A"), leg("B"), leg("A")}, []string{"A", "B", "A"}},
		{"empty summaries are dropped", []*justrouting.Leg{leg(""), leg("A")}, []string{"A"}},
		{"whitespace is trimmed", []*justrouting.Leg{leg("  A  "), leg("A")}, []string{"A"}},
		{"nil legs are skipped", []*justrouting.Leg{nil, leg("A")}, []string{"A"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := majorRoads(tt.legs)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("unexpected roads: got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRoadClasses(t *testing.T) {
	intersection := func(classes ...string) *justrouting.Intersection {
		return &justrouting.Intersection{Classes: classes}
	}
	step := func(intersections ...*justrouting.Intersection) *justrouting.Step {
		return &justrouting.Step{Intersections: intersections}
	}
	leg := func(steps ...*justrouting.Step) *justrouting.Leg {
		return &justrouting.Leg{Steps: steps}
	}

	tests := []struct {
		name      string
		legs      []*justrouting.Leg
		wantTolls bool
		wantFerry bool
	}{
		{"no legs", nil, false, false},
		{"empty steps", []*justrouting.Leg{leg()}, false, false},
		{"no intersections", []*justrouting.Leg{leg(step())}, false, false},
		{"toll class", []*justrouting.Leg{leg(step(intersection("toll")))}, true, false},
		{"ferry class", []*justrouting.Leg{leg(step(intersection("ferry")))}, false, true},
		{"both classes", []*justrouting.Leg{leg(step(intersection("toll", "ferry")))}, true, true},
		{"any step counts", []*justrouting.Leg{leg(step(), step(intersection("toll")))}, true, false},
		{"nil legs and steps are skipped", []*justrouting.Leg{nil, leg(nil, step(nil, intersection("ferry")))}, false, true},
		{"class matching is case-insensitive", []*justrouting.Leg{leg(step(intersection("TOLL")))}, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tolls, ferry := roadClasses(tt.legs)
			if tolls != tt.wantTolls || ferry != tt.wantFerry {
				t.Fatalf("unexpected classes: got tolls=%v ferry=%v, want tolls=%v ferry=%v",
					tolls, ferry, tt.wantTolls, tt.wantFerry)
			}
		})
	}
}

func TestBuildRoutePoint(t *testing.T) {
	input := justrouting.Point{103.8198, 1.3521}

	// A nil waypoint leaves only the echoed input coordinate.
	out := buildRoutePoint(input, nil)
	if out.Input != "103.8198,1.3521" {
		t.Fatalf("unexpected input: %q", out.Input)
	}
	if len(out.Snapped) != 0 || out.Name != "" || out.Distance != 0 {
		t.Fatalf("expected no snapping data, got: %+v", out)
	}

	// A present waypoint maps every field.
	out = buildRoutePoint(input, &justrouting.Waypoint{
		Name:     "Marina Boulevard",
		Location: justrouting.Point{103.81982, 1.35211},
		Distance: 4.216,
	})
	if out.Input != "103.8198,1.3521" {
		t.Fatalf("unexpected input: %q", out.Input)
	}
	if out.Name != "Marina Boulevard" {
		t.Fatalf("unexpected name: %q", out.Name)
	}
	if out.Snapped.Lon() != 103.81982 || out.Snapped.Lat() != 1.35211 {
		t.Fatalf("unexpected snapped: %v", out.Snapped)
	}
	if out.Distance != 4.216 {
		t.Fatalf("unexpected distance: %v", out.Distance)
	}
}
