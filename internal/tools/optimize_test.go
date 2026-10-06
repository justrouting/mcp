package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	justrouting "github.com/justrouting/go-client"
)

// optimizeFixture is the go-client's solutionFixture verbatim. Its extra
// delivery/pickup/load fields decode fine into the go-client structs and are
// simply not copied onto our trimmed output structs.
const optimizeFixture = `{
  "code": 0,
  "summary": {
    "cost": 2841, "routes": 1, "unassigned": 1,
    "delivery": [3], "pickup": [0],
    "setup": 0, "service": 900, "duration": 2841,
    "waiting_time": 0, "priority": 0, "distance": 31204
  },
  "unassigned": [
    {"id": 4, "location": [103.7, 1.42], "type": "job", "description": "far depot"}
  ],
  "routes": [
    {
      "vehicle": 1, "cost": 2841, "setup": 0, "service": 900,
      "duration": 2841, "waiting_time": 0, "priority": 0, "distance": 31204,
      "delivery": [3], "pickup": [0],
      "steps": [
        {"type":"start","location":[103.8198,1.3521],"setup":0,"service":0,"waiting_time":0,"arrival":0,"duration":0,"distance":0},
        {"type":"job","location":[103.8514,1.2897],"id":1,"job":1,"setup":0,"service":300,"waiting_time":0,"arrival":842,"duration":842,"distance":9120,"load":[2]},
        {"type":"end","location":[103.8198,1.3521],"setup":0,"service":0,"waiting_time":0,"arrival":2841,"duration":2841,"distance":31204}
      ]
    }
  ]
}`

// optimizeEmptySolution is a minimal success body for request-shape tests.
const optimizeEmptySolution = `{"code":0,"summary":{},"routes":[],"unassigned":[]}`

// optimizeTestInput is a valid one-vehicle, one-job request.
func optimizeTestInput() OptimizeInput {
	return OptimizeInput{
		Vehicles: []OptimizeVehicleInput{
			{ID: 1, Start: "103.8198,1.3521", End: "103.8198,1.3521"},
		},
		Jobs: []OptimizeJobInput{
			{ID: 1, Location: "103.8514,1.2897"},
		},
	}
}

func TestGetOptimizeDecodesSolution(t *testing.T) {
	client := newTestClient(t, jsonHandler(http.StatusOK, optimizeFixture))

	_, output, err := getOptimize(
		context.Background(),
		client,
		optimizeTestInput(),
	)
	if err != nil {
		t.Fatalf("getOptimize: %v", err)
	}

	if output.Summary.Cost != 2841 {
		t.Errorf("Summary.Cost = %d, want 2841", output.Summary.Cost)
	}
	if output.Summary.Distance != 31204 {
		t.Errorf("Summary.Distance = %d, want 31204", output.Summary.Distance)
	}
	if len(output.Routes) != 1 {
		t.Fatalf("len(Routes) = %d, want 1", len(output.Routes))
	}
	route := output.Routes[0]
	if route.Vehicle != 1 {
		t.Errorf("Vehicle = %d, want 1", route.Vehicle)
	}
	if len(route.Steps) != 3 {
		t.Fatalf("len(Steps) = %d, want 3", len(route.Steps))
	}
	if route.Steps[0].Type != "start" || route.Steps[2].Type != "end" {
		t.Errorf("steps = %q..%q, want start..end", route.Steps[0].Type, route.Steps[2].Type)
	}
	if job := route.Steps[1]; job.Job != 1 || job.Arrival != 842 {
		t.Errorf("job step = {Job:%d Arrival:%d}, want {1 842}", job.Job, job.Arrival)
	}
	if lon := route.Steps[1].Location.Lon(); lon != 103.8514 {
		t.Errorf("job step location lon = %v, want 103.8514", lon)
	}
	if len(output.Unassigned) != 1 {
		t.Fatalf("len(Unassigned) = %d, want 1", len(output.Unassigned))
	}
	if output.Unassigned[0].ID != 4 {
		t.Errorf("Unassigned[0].ID = %d, want 4", output.Unassigned[0].ID)
	}
}

func TestGetOptimizeRequestBody(t *testing.T) {
	newBodyCapturingClient := func(t *testing.T) (*justrouting.Client, *map[string]any) {
		t.Helper()
		body := make(map[string]any)
		client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			if r.URL.Path != "/optimize" {
				t.Errorf("path = %q, want /optimize", r.URL.Path)
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(optimizeEmptySolution))
		})
		return client, &body
	}

	t.Run("round trip with default profile", func(t *testing.T) {
		client, body := newBodyCapturingClient(t)

		_, _, err := getOptimize(context.Background(), client, OptimizeInput{
			Vehicles: []OptimizeVehicleInput{
				{ID: 1, Profile: "driving", Start: "103.8198,1.3521"},
			},
			Jobs: []OptimizeJobInput{
				{ID: 1, Location: "103.8514,1.2897"},
			},
		})
		if err != nil {
			t.Fatalf("getOptimize: %v", err)
		}

		vehicle := (*body)["vehicles"].([]any)[0].(map[string]any)
		if got := vehicle["id"].(float64); got != 1 {
			t.Errorf("vehicles[0].id = %v, want 1", got)
		}
		if got := vehicle["profile"].(string); got != "car" {
			t.Errorf("vehicles[0].profile = %q, want \"car\"", got)
		}
		if got := vehicle["start"]; !reflect.DeepEqual(got, []any{103.8198, 1.3521}) {
			t.Errorf("vehicles[0].start = %v, want [103.8198 1.3521]", got)
		}
		if _, ok := vehicle["end"]; ok {
			t.Error("vehicles[0].end should be omitted when empty")
		}

		job := (*body)["jobs"].([]any)[0].(map[string]any)
		if got := job["location"]; !reflect.DeepEqual(got, []any{103.8514, 1.2897}) {
			t.Errorf("jobs[0].location = %v, want [103.8514 1.2897]", got)
		}

		if _, ok := (*body)["shipments"]; ok {
			t.Error("shipments should be omitted when empty")
		}
	})

	t.Run("motorcycle profile", func(t *testing.T) {
		client, body := newBodyCapturingClient(t)

		_, _, err := getOptimize(context.Background(), client, OptimizeInput{
			Vehicles: []OptimizeVehicleInput{
				{ID: 1, Profile: "motorcycle", Start: "103.8198,1.3521"},
			},
			Jobs: []OptimizeJobInput{
				{ID: 1, Location: "103.8514,1.2897"},
			},
		})
		if err != nil {
			t.Fatalf("getOptimize: %v", err)
		}

		vehicle := (*body)["vehicles"].([]any)[0].(map[string]any)
		if got := vehicle["profile"].(string); got != "motorcycle" {
			t.Errorf("vehicles[0].profile = %q, want \"motorcycle\"", got)
		}
	})

	t.Run("empty start means anywhere", func(t *testing.T) {
		client, body := newBodyCapturingClient(t)

		_, _, err := getOptimize(context.Background(), client, OptimizeInput{
			Vehicles: []OptimizeVehicleInput{
				{ID: 1, Start: "", End: "103.8198,1.3521"},
			},
			Jobs: []OptimizeJobInput{
				{ID: 1, Location: "103.8514,1.2897"},
			},
		})
		if err != nil {
			t.Fatalf("getOptimize: %v", err)
		}

		vehicle := (*body)["vehicles"].([]any)[0].(map[string]any)
		if _, ok := vehicle["start"]; ok {
			t.Error("vehicles[0].start should be omitted when empty")
		}
		if _, ok := vehicle["end"]; !ok {
			t.Error("vehicles[0].end should be present")
		}
		if got := vehicle["profile"].(string); got != "car" {
			t.Errorf("vehicles[0].profile = %q, want \"car\" (the default)", got)
		}
	})
}

func TestGetOptimizeInputValidation(t *testing.T) {
	validVehicle := func() OptimizeVehicleInput {
		return OptimizeVehicleInput{ID: 1, Start: "103.8198,1.3521"}
	}
	validJob := func() OptimizeJobInput {
		return OptimizeJobInput{ID: 1, Location: "103.8514,1.2897"}
	}

	tests := []struct {
		name      string
		input     OptimizeInput
		wantError string
	}{
		{"no vehicles", OptimizeInput{Jobs: []OptimizeJobInput{validJob()}}, "at least 1 vehicle"},
		{"no jobs", OptimizeInput{Vehicles: []OptimizeVehicleInput{validVehicle()}}, "at least 1 job"},
		{
			"vehicle without start or end",
			OptimizeInput{
				Vehicles: []OptimizeVehicleInput{{ID: 1}},
				Jobs:     []OptimizeJobInput{validJob()},
			},
			"vehicles[0]",
		},
		{
			"malformed vehicle start reports its position",
			OptimizeInput{
				Vehicles: []OptimizeVehicleInput{{ID: 1, Start: "not-a-coordinate"}},
				Jobs:     []OptimizeJobInput{validJob()},
			},
			"vehicles[0].start",
		},
		{
			"malformed job location reports its position",
			OptimizeInput{
				Vehicles: []OptimizeVehicleInput{validVehicle()},
				Jobs: []OptimizeJobInput{
					validJob(),
					{ID: 2, Location: "not-a-coordinate"},
				},
			},
			"jobs[1].location",
		},
		{
			"unsupported profile",
			OptimizeInput{
				Vehicles: []OptimizeVehicleInput{{ID: 1, Profile: "walking", Start: "103.8198,1.3521"}},
				Jobs:     []OptimizeJobInput{validJob()},
			},
			"vehicles[0].profile",
		},
		{
			"dropped car synonym",
			OptimizeInput{
				Vehicles: []OptimizeVehicleInput{{ID: 1, Profile: "car", Start: "103.8198,1.3521"}},
				Jobs:     []OptimizeJobInput{validJob()},
			},
			"vehicles[0].profile",
		},
		{
			"duplicate vehicle ids",
			OptimizeInput{
				Vehicles: []OptimizeVehicleInput{
					validVehicle(),
					{ID: 1, Start: "103.84,1.30"},
				},
				Jobs: []OptimizeJobInput{validJob()},
			},
			"vehicle ids must be unique",
		},
		{
			"duplicate job ids",
			OptimizeInput{
				Vehicles: []OptimizeVehicleInput{validVehicle()},
				Jobs: []OptimizeJobInput{
					validJob(),
					{ID: 1, Location: "103.84,1.30"},
				},
			},
			"job ids must be unique",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				t.Error("no request should reach the server")
			})

			if _, _, err := getOptimize(
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

func TestGetOptimizeAPIErrorPassthrough(t *testing.T) {
	t.Run("unauthorized", func(t *testing.T) {
		client := newTestClient(
			t,
			jsonHandler(
				http.StatusUnauthorized,
				`{"statusCode":401,"error":"Unauthorized","message":"Invalid apiKey"}`,
			),
		)

		_, _, err := getOptimize(context.Background(), client, optimizeTestInput())
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, justrouting.ErrUnauthorized) {
			t.Fatalf("expected ErrUnauthorized, got: %v", err)
		}
		if !strings.Contains(err.Error(), "optimization failed") {
			t.Fatalf("expected wrapped error, got: %v", err)
		}
	})

	t.Run("plan limit exceeded", func(t *testing.T) {
		client := newTestClient(
			t,
			jsonHandler(
				http.StatusBadRequest,
				`{"error":"too many jobs"}`,
			),
		)

		_, _, err := getOptimize(context.Background(), client, optimizeTestInput())
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, justrouting.ErrPlanLimitExceeded) {
			t.Fatalf("expected ErrPlanLimitExceeded, got: %v", err)
		}
	})

	t.Run("engine failure behind HTTP 200", func(t *testing.T) {
		client := newTestClient(
			t,
			jsonHandler(
				http.StatusOK,
				`{"code":2,"error":"Invalid ID"}`,
			),
		)

		_, _, err := getOptimize(context.Background(), client, optimizeTestInput())
		if err == nil {
			t.Fatal("expected error")
		}
		var apiErr *justrouting.Error
		if !errors.As(err, &apiErr) {
			t.Fatalf("expected *justrouting.Error, got: %v", err)
		}
		if apiErr.VROOMCode != 2 {
			t.Errorf("VROOMCode = %d, want 2", apiErr.VROOMCode)
		}
	})
}

func TestOptimizeProfile(t *testing.T) {
	tests := []struct {
		profile Profile
		want    string
	}{
		{ProfileDriving, "car"},
		{"", "car"},
		{ProfileMotorcycle, "motorcycle"},
	}

	for _, tt := range tests {
		t.Run(string(tt.profile), func(t *testing.T) {
			if got := optimizeProfile(tt.profile); got != tt.want {
				t.Errorf("optimizeProfile(%q) = %q, want %q", tt.profile, got, tt.want)
			}
		})
	}
}
