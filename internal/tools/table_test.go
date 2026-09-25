package tools

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	justrouting "github.com/justrouting/go-client"
)

// tableFixture includes a null entry, which is how the engine reports a pair
// it cannot connect. It must stay null in the tool output, not become zero.
const tableFixture = `{
  "code": "Ok",
  "durations": [[0, 612.4, 1583.4], [598.1, 0, null], [1601.2, null, 0]],
  "distances": [[0, 8421.5, 24512.7], [8390.2, 0, null], [24488.1, null, 0]],
  "sources": [
    {"hint":"a","distance":4.2,"name":"Marina Boulevard","location":[103.81982,1.35211]},
    {"hint":"b","distance":2.1,"name":"Orchard Road","location":[103.83,1.3048]},
    {"hint":"c","distance":8.1,"name":"Changi Coast Road","location":[103.99151,1.36442]}
  ],
  "destinations": [
    {"hint":"a","distance":4.2,"name":"Marina Boulevard","location":[103.81982,1.35211]},
    {"hint":"b","distance":2.1,"name":"Orchard Road","location":[103.83,1.3048]},
    {"hint":"c","distance":8.1,"name":"Changi Coast Road","location":[103.99151,1.36442]}
  ]
}`

// tableRectangularFixture is the response for a request restricted to one
// source (coordinate 0) and two destinations (coordinates 2 and 1, in that
// order). The waypoint order mirrors the request's selection order.
const tableRectangularFixture = `{
  "code": "Ok",
  "durations": [[612.4, 1583.4]],
  "distances": [[8421.5, 24512.7]],
  "sources": [
    {"hint":"a","distance":4.2,"name":"Marina Boulevard","location":[103.81982,1.35211]}
  ],
  "destinations": [
    {"hint":"c","distance":8.1,"name":"Changi Coast Road","location":[103.99151,1.36442]},
    {"hint":"b","distance":2.1,"name":"Orchard Road","location":[103.83,1.3048]}
  ]
}`

func threeCoordinateStrings() []string {
	return []string{"103.8198,1.3521", "103.83,1.3048", "103.9915,1.3644"}
}

func TestGetTableDecodesMatrix(t *testing.T) {
	client := newTestClient(t, jsonHandler(http.StatusOK, tableFixture))

	_, output, err := getTable(
		context.Background(),
		client,
		TableInput{Coordinates: threeCoordinateStrings()},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if output.Code != "Ok" {
		t.Fatalf("unexpected code: %q", output.Code)
	}

	if len(output.Durations) != 3 || len(output.Durations[0]) != 3 {
		t.Fatalf("unexpected durations shape: %v", output.Durations)
	}
	if output.Durations[0][1] == nil {
		t.Fatal("unexpected nil duration for a reachable pair")
	}
	if got := *output.Durations[0][1]; got != 612.4 {
		t.Fatalf("unexpected duration[0][1]: %v", got)
	}
	if output.Distances[0][2] == nil {
		t.Fatal("unexpected nil distance for a reachable pair")
	}
	if got := *output.Distances[0][2]; got != 24512.7 {
		t.Fatalf("unexpected distance[0][2]: %v", got)
	}

	// The null entry must stay null so the LLM can tell an unreachable pair
	// apart from a genuine zero.
	if output.Durations[1][2] != nil {
		t.Fatalf("expected null duration for unreachable pair, got %v", output.Durations[1][2])
	}

	if len(output.Sources) != 3 {
		t.Fatalf("unexpected sources: %+v", output.Sources)
	}
	for i, wantIndex := range []int{0, 1, 2} {
		if output.Sources[i].Index != wantIndex {
			t.Fatalf(
				"unexpected sources[%d].Index: %d, want %d",
				i, output.Sources[i].Index, wantIndex,
			)
		}
	}
	if output.Sources[0].Name != "Marina Boulevard" {
		t.Fatalf("unexpected sources[0].Name: %q", output.Sources[0].Name)
	}
	if lon := output.Sources[0].Location.Lon(); lon != 103.81982 {
		t.Fatalf("unexpected sources[0].Location.Lon: %v", lon)
	}
	if output.Sources[0].Distance != 4.2 {
		t.Fatalf("unexpected sources[0].Distance: %v", output.Sources[0].Distance)
	}

	if len(output.Destinations) != 3 {
		t.Fatalf("unexpected destinations: %+v", output.Destinations)
	}
	if output.Destinations[2].Index != 2 {
		t.Fatalf("unexpected destinations[2].Index: %d", output.Destinations[2].Index)
	}
}

func TestGetTableSourcesDestinationsIndexMapping(t *testing.T) {
	var gotQuery url.Values
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(tableRectangularFixture))
	})

	// Destinations are deliberately not sorted to prove the reported index
	// comes from the request's selection order, not the response position.
	_, output, err := getTable(
		context.Background(),
		client,
		TableInput{
			Coordinates:  threeCoordinateStrings(),
			Sources:      []int{0},
			Destinations: []int{2, 1},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gotQuery.Get("sources"); got != "0" {
		t.Fatalf("unexpected sources query: got %q", got)
	}
	if got := gotQuery.Get("destinations"); got != "2;1" {
		t.Fatalf("unexpected destinations query: got %q", got)
	}

	if len(output.Sources) != 1 {
		t.Fatalf("unexpected sources: %+v", output.Sources)
	}
	if output.Sources[0].Index != 0 {
		t.Fatalf("unexpected sources[0].Index: %d, want 0", output.Sources[0].Index)
	}

	if len(output.Destinations) != 2 {
		t.Fatalf("unexpected destinations: %+v", output.Destinations)
	}
	if output.Destinations[0].Index != 2 {
		t.Fatalf("unexpected destinations[0].Index: %d, want 2", output.Destinations[0].Index)
	}
	if output.Destinations[1].Index != 1 {
		t.Fatalf("unexpected destinations[1].Index: %d, want 1", output.Destinations[1].Index)
	}
	if output.Destinations[0].Name != "Changi Coast Road" {
		t.Fatalf("unexpected destinations[0].Name: %q", output.Destinations[0].Name)
	}
}

func TestGetTableQueryEncoding(t *testing.T) {
	tests := []struct {
		name       string
		input      TableInput
		wantPath   string
		wantQuery  map[string]string
		wantAbsent []string
	}{
		{
			name:     "defaults compute the full matrix with both annotations",
			input:    TableInput{Coordinates: threeCoordinateStrings()},
			wantPath: "/table/v1/driving/103.8198,1.3521;103.83,1.3048;103.9915,1.3644",
			wantQuery: map[string]string{
				"annotations": "duration,distance",
			},
			wantAbsent: []string{"sources", "destinations"},
		},
		{
			name: "sources and destinations restrict to a rectangular subset",
			input: TableInput{
				Coordinates:  threeCoordinateStrings(),
				Sources:      []int{0},
				Destinations: []int{1, 2},
			},
			wantPath: "/table/v1/driving/103.8198,1.3521;103.83,1.3048;103.9915,1.3644",
			wantQuery: map[string]string{
				"annotations":  "duration,distance",
				"sources":      "0",
				"destinations": "1;2",
			},
		},
		{
			name:     "a single annotation is forwarded",
			input:    TableInput{Coordinates: threeCoordinateStrings(), Annotations: []string{"distance"}},
			wantPath: "/table/v1/driving/103.8198,1.3521;103.83,1.3048;103.9915,1.3644",
			wantQuery: map[string]string{
				"annotations": "distance",
			},
		},
		{
			name:     "motorcycle profile goes into the path",
			input:    TableInput{Coordinates: threeCoordinateStrings(), Profile: "motorcycle"},
			wantPath: "/table/v1/motorcycle/103.8198,1.3521;103.83,1.3048;103.9915,1.3644",
			wantQuery: map[string]string{
				"annotations": "duration,distance",
			},
		},
		{
			name:     "car profile normalizes to driving",
			input:    TableInput{Coordinates: threeCoordinateStrings(), Profile: "car"},
			wantPath: "/table/v1/driving/103.8198,1.3521;103.83,1.3048;103.9915,1.3644",
			wantQuery: map[string]string{
				"annotations": "duration,distance",
			},
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
				_, _ = w.Write([]byte(`{"code":"Ok"}`))
			})

			if _, _, err := getTable(
				context.Background(),
				client,
				tt.input,
			); err != nil {
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
		})
	}
}

func TestGetTableInputValidation(t *testing.T) {
	tests := []struct {
		name      string
		input     TableInput
		wantError string
	}{
		{"no coordinates", TableInput{}, "at least 2 coordinates"},
		{
			"single coordinate",
			TableInput{Coordinates: []string{"103.8198,1.3521"}},
			"at least 2 coordinates",
		},
		{
			"malformed coordinate reports its position",
			TableInput{Coordinates: []string{"103.8198,1.3521", "not-a-coordinate", "103.9915,1.3644"}},
			"coordinates[1]",
		},
		{
			"unsupported profile",
			TableInput{Coordinates: threeCoordinateStrings(), Profile: "walking"},
			"invalid profile",
		},
		{
			"source index out of range",
			TableInput{Coordinates: threeCoordinateStrings(), Sources: []int{5}},
			"invalid sources[0]",
		},
		{
			"negative destination index",
			TableInput{Coordinates: threeCoordinateStrings(), Destinations: []int{-1}},
			"invalid destinations[0]",
		},
		{
			"invalid annotation",
			TableInput{Coordinates: threeCoordinateStrings(), Annotations: []string{"speed"}},
			"invalid annotation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("no request should reach the server")
			})

			if _, _, err := getTable(
				context.Background(),
				client,
				tt.input,
			); err == nil {
				t.Fatalf("expected error for %+v", tt.input)
			} else if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetTableAPIErrorPassthrough(t *testing.T) {
	t.Run("unauthorized", func(t *testing.T) {
		client := newTestClient(
			t,
			jsonHandler(
				http.StatusUnauthorized,
				`{"statusCode":401,"error":"Unauthorized","message":"Invalid apiKey"}`,
			),
		)

		_, _, err := getTable(
			context.Background(),
			client,
			TableInput{Coordinates: threeCoordinateStrings()},
		)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, justrouting.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got: %v", err)
		}
		if !strings.Contains(err.Error(), "table calculation failed") {
			t.Fatalf("expected wrapped error, got: %v", err)
		}
	})

	t.Run("plan limit exceeded", func(t *testing.T) {
		client := newTestClient(
			t,
			jsonHandler(
				http.StatusBadRequest,
				`{"error":"matrix size exceeds plan limit"}`,
			),
		)

		_, _, err := getTable(
			context.Background(),
			client,
			TableInput{Coordinates: threeCoordinateStrings()},
		)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, justrouting.ErrPlanLimitExceeded) {
			t.Fatalf("expected ErrPlanLimitExceeded, got: %v", err)
		}
	})
}

func TestNormalizeAnnotations(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil stays nil so the client default applies", nil, nil},
		{"empty stays nil", []string{}, nil},
		{"single annotation", []string{"duration"}, []string{"duration"}},
		{
			"case is normalized and duplicates dropped",
			[]string{"Distance", " distance "},
			[]string{"distance"},
		},
		{
			"order is preserved",
			[]string{"duration", "distance"},
			[]string{"duration", "distance"},
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
	for _, in := range [][]string{{"speed"}, {"duration", "walking"}} {
		if _, err := normalizeAnnotations(in); err == nil {
			t.Fatalf("expected error for %v", in)
		}
	}
}

func TestEffectiveIndices(t *testing.T) {
	tests := []struct {
		name string
		sel  []int
		n    int
		want []int
	}{
		{"empty selection expands to all", nil, 3, []int{0, 1, 2}},
		{"non-empty selection passes through", []int{2, 0}, 3, []int{2, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveIndices(tt.sel, tt.n)
			if len(got) != len(tt.want) {
				t.Fatalf("unexpected indices: %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("unexpected indices: %v, want %v", got, tt.want)
				}
			}
		})
	}
}
