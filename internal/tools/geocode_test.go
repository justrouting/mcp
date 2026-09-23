package tools

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	justrouting "github.com/justrouting/go-client"
)

const testAPIKey = "test-api-key"

const geocodeFixture = `{
  "results": [{
    "name": "Marina Bay Sands",
    "country_code": "sg",
    "lon": 103.859,
    "lat": 1.2834,
    "formatted": "Marina Bay Sands, 10 Bayfront Avenue, 018956, Singapore",
    "result_type": "building",
    "place_id": "51667b3e1416f7594059aad55757058af43ff00103f9018322790001000000c0"
  }]
}`

// newTestClient starts a server running handler and returns a client aimed at
// it. Backoff is zeroed so retry paths run instantly.
func newTestClient(t *testing.T, handler http.HandlerFunc) *justrouting.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return justrouting.NewClient(
		testAPIKey,
		justrouting.WithBaseURL(srv.URL),
		justrouting.WithBackoff(func(int) time.Duration { return 0 }),
	)
}

// jsonHandler replies with a fixed status and body.
func jsonHandler(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestSearchGeocodeDecodesResults(t *testing.T) {
	client := newTestClient(t, jsonHandler(http.StatusOK, geocodeFixture))

	_, output, err := searchGeocode(
		context.Background(),
		client,
		GeocodeInput{Name: "marina bay sands", City: "singapore"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(output.Results) != 1 {
		t.Fatalf("unexpected results: %+v", output.Results)
	}

	result := output.Results[0]
	if result.Longitude != 103.859 {
		t.Fatalf("unexpected longitude: %v", result.Longitude)
	}
	if result.Latitude != 1.2834 {
		t.Fatalf("unexpected latitude: %v", result.Latitude)
	}

	// The coordinates string is what the route tool consumes; it must be
	// ready to paste into origin/destination as-is.
	if result.Coordinates != "103.859,1.2834" {
		t.Fatalf("unexpected coordinates: %q", result.Coordinates)
	}

	if result.Formatted != "Marina Bay Sands, 10 Bayfront Avenue, 018956, Singapore" {
		t.Fatalf("unexpected formatted: %q", result.Formatted)
	}
	if result.PlaceID != "51667b3e1416f7594059aad55757058af43ff00103f9018322790001000000c0" {
		t.Fatalf("unexpected place_id: %q", result.PlaceID)
	}
	if result.CountryCode != "sg" {
		t.Fatalf("unexpected country_code: %q", result.CountryCode)
	}
	if result.ResultType != "building" {
		t.Fatalf("unexpected result_type: %q", result.ResultType)
	}
}

func TestSearchGeocodeEmptyResults(t *testing.T) {
	client := newTestClient(t, jsonHandler(http.StatusOK, `{"results":[]}`))

	_, _, err := searchGeocode(
		context.Background(),
		client,
		GeocodeInput{Name: "nowhere at all"},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no results found") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), `name="nowhere at all"`) {
		t.Fatalf("error should echo the searched components: %v", err)
	}
}

func TestSearchGeocodeInputValidation(t *testing.T) {
	tests := []struct {
		name  string
		input GeocodeInput
	}{
		{"all fields empty", GeocodeInput{}},
		{
			"whitespace only fields",
			GeocodeInput{Name: "  ", City: "\t", Country: " "},
		},
		{"negative limit", GeocodeInput{Name: "x", Limit: -1}},
		{"limit too large", GeocodeInput{Name: "x", Limit: maxGeocodeResults + 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("no request should reach the server")
			})

			if _, _, err := searchGeocode(
				context.Background(),
				client,
				tt.input,
			); err == nil {
				t.Fatalf("expected error for %+v", tt.input)
			}
		})
	}
}

func TestSearchGeocodeQueryEncoding(t *testing.T) {
	tests := []struct {
		name       string
		input      GeocodeInput
		wantSet    map[string]string
		wantAbsent []string
	}{
		{
			name:  "only determined components are sent, omitted limit defaults to 1",
			input: GeocodeInput{Name: "marina bay", Country: "singapore"},
			wantSet: map[string]string{
				"name":    "marina bay",
				"country": "singapore",
				"limit":   "1",
			},
			wantAbsent: []string{"housenumber", "street", "postcode", "city", "text"},
		},
		{
			name: "full address forwards every component",
			input: GeocodeInput{
				Name:        "Marina Bay Sands",
				Housenumber: "10",
				Street:      "Bayfront Avenue",
				Postcode:    "018956",
				City:        "Singapore",
				Country:     "Singapore",
				Limit:       3,
			},
			wantSet: map[string]string{
				"name":        "Marina Bay Sands",
				"housenumber": "10",
				"street":      "Bayfront Avenue",
				"postcode":    "018956",
				"city":        "Singapore",
				"country":     "Singapore",
				"limit":       "3",
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
				_, _ = w.Write([]byte(`{"results":[{"lon":103.8,"lat":1.3}]}`))
			})

			if _, _, err := searchGeocode(
				context.Background(),
				client,
				tt.input,
			); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotPath != "/geocode/v1/search" {
				t.Fatalf("unexpected path: got %q, want %q", gotPath, "/geocode/v1/search")
			}

			for key, want := range tt.wantSet {
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

func TestSearchGeocodeAPIErrorPassthrough(t *testing.T) {
	client := newTestClient(
		t,
		jsonHandler(
			http.StatusUnauthorized,
			`{"statusCode":401,"error":"Unauthorized","message":"Invalid apiKey"}`,
		),
	)

	_, _, err := searchGeocode(
		context.Background(),
		client,
		GeocodeInput{City: "Singapore"},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, justrouting.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
	if !strings.Contains(err.Error(), "geocode search failed") {
		t.Fatalf("expected wrapped error, got: %v", err)
	}
}
