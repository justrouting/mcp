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

// optimizeFixture is the go-client's solutionFixture plus the fields the
// engine emits for richer inputs: violations, location_index, route
// description and geometry.
const optimizeFixture = `{
  "code": 0,
  "summary": {
    "cost": 2841, "routes": 1, "unassigned": 1,
    "delivery": [3], "pickup": [0],
    "setup": 0, "service": 900, "duration": 2841,
    "waiting_time": 0, "priority": 0, "distance": 31204,
    "violations": []
  },
  "unassigned": [
    {"id": 4, "location": [103.7, 1.42], "location_index": 3, "type": "job", "description": "far depot"}
  ],
  "routes": [
    {
      "vehicle": 1, "cost": 2841, "setup": 0, "service": 900,
      "duration": 2841, "waiting_time": 0, "priority": 0, "distance": 31204,
      "delivery": [3], "pickup": [0],
      "description": "van one", "geometry": "encoded_polyline", "violations": [],
      "steps": [
        {"type":"start","location":[103.8198,1.3521],"setup":0,"service":0,"waiting_time":0,"arrival":0,"duration":0,"distance":0},
        {"type":"job","location":[103.8514,1.2897],"location_index":1,"id":1,"job":1,"setup":0,"service":300,"waiting_time":0,"arrival":842,"duration":842,"distance":9120,"load":[2]},
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
		Vehicles: []OptimizeVehicle{
			{ID: 1, Start: "103.8198,1.3521", End: "103.8198,1.3521"},
		},
		Jobs: []OptimizeJob{
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
	if output.Unassigned[0].LocationIndex != 3 {
		t.Errorf("Unassigned[0].LocationIndex = %d, want 3", output.Unassigned[0].LocationIndex)
	}

	if len(output.Summary.Delivery) != 1 || output.Summary.Delivery[0] != 3 {
		t.Errorf("Summary.Delivery = %v, want [3]", output.Summary.Delivery)
	}
	if len(output.Summary.Violations) != 0 {
		t.Errorf("Summary.Violations = %v, want empty", output.Summary.Violations)
	}
	if route.Description != "van one" {
		t.Errorf("Description = %q, want \"van one\"", route.Description)
	}
	if route.Geometry != "encoded_polyline" {
		t.Errorf("Geometry = %q, want \"encoded_polyline\"", route.Geometry)
	}
	if job := route.Steps[1]; job.ID != 1 || job.LocationIndex != 1 || len(job.Load) != 1 || job.Load[0] != 2 {
		t.Errorf("job step = {ID:%d LocationIndex:%d Load:%v}, want {1 1 [2]}",
			job.ID, job.LocationIndex, job.Load)
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
			Vehicles: []OptimizeVehicle{
				{ID: 1, Profile: "driving", Start: "103.8198,1.3521"},
			},
			Jobs: []OptimizeJob{
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
			Vehicles: []OptimizeVehicle{
				{ID: 1, Profile: "motorcycle", Start: "103.8198,1.3521"},
			},
			Jobs: []OptimizeJob{
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
			Vehicles: []OptimizeVehicle{
				{ID: 1, Start: "", End: "103.8198,1.3521"},
			},
			Jobs: []OptimizeJob{
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

	t.Run("full request forwards every new field", func(t *testing.T) {
		client, body := newBodyCapturingClient(t)

		timeWindow := OptimizeTimeWindow{28800, 57600}
		_, _, err := getOptimize(context.Background(), client, OptimizeInput{
			Vehicles: []OptimizeVehicle{
				{
					ID:            1,
					Description:   "van one",
					Start:         "103.8198,1.3521",
					End:           "103.8198,1.3521",
					Capacity:      []int{4},
					Skills:        []int{1},
					Type:          "van",
					TimeWindow:    &timeWindow,
					SpeedFactor:   0.8,
					MaxTasks:      3,
					MaxTravelTime: 14400,
					MaxDistance:   50000,
					Breaks: []OptimizeBreak{
						{ID: 1, TimeWindows: []OptimizeTimeWindow{{36000, 39600}}, Service: 1800, MaxLoad: []int{2}},
					},
					Costs: &OptimizeVehicleCosts{Fixed: 10, PerHour: 2000, PerTaskHour: 300, PerKM: 2},
					Steps: []OptimizeVehicleStep{{Type: "job", ID: 1}},
				},
			},
			Jobs: []OptimizeJob{
				{
					ID:             1,
					Description:    "first stop",
					Location:       "103.8514,1.2897",
					Setup:          60,
					Service:        300,
					SetupPerType:   map[string]int{"van": 120},
					ServicePerType: map[string]int{"van": 600},
					Delivery:       []int{1},
					Skills:         []int{1},
					Priority:       50,
					TimeWindows:    []OptimizeTimeWindow{{0, 14400}},
				},
			},
			Shipments: []OptimizeShipment{
				{
					Pickup: OptimizeShipmentStep{
						ID: 2, Location: "103.85,1.29", Setup: 30, Service: 150,
						SetupPerType:   map[string]int{"van": 60},
						ServicePerType: map[string]int{"van": 300},
						TimeWindows:    []OptimizeTimeWindow{{7200, 21600}},
					},
					Delivery: OptimizeShipmentStep{
						ID: 3, Location: "103.9,1.31", Service: 150,
					},
					Amount:   []int{2},
					Skills:   []int{1},
					Priority: 75,
				},
			},
			Matrices: map[string]OptimizeMatrix{
				"car": {
					Durations: [][]int{{0, 14}, {21, 0}},
					Distances: [][]int{{0, 1500}, {1500, 0}},
					Costs:     [][]int{{0, 1}, {1, 0}},
				},
			},
			Options: &OptimizeOptions{Geometry: true},
		})
		if err != nil {
			t.Fatalf("getOptimize: %v", err)
		}

		vehicle := (*body)["vehicles"].([]any)[0].(map[string]any)
		if vehicle["description"] != "van one" {
			t.Errorf("vehicle description = %v, want \"van one\"", vehicle["description"])
		}
		if tw, ok := vehicle["time_window"].([]any); !ok || tw[0].(float64) != 28800 || tw[1].(float64) != 57600 {
			t.Errorf("time_window = %v, want [28800, 57600]", vehicle["time_window"])
		}
		if vehicle["type"] != "van" || vehicle["speed_factor"].(float64) != 0.8 {
			t.Errorf("type/speed_factor = %v/%v, want van/0.8", vehicle["type"], vehicle["speed_factor"])
		}
		if vehicle["max_tasks"].(float64) != 3 || vehicle["max_travel_time"].(float64) != 14400 ||
			vehicle["max_distance"].(float64) != 50000 {
			t.Errorf("limits = %v/%v/%v, want 3/14400/50000",
				vehicle["max_tasks"], vehicle["max_travel_time"], vehicle["max_distance"])
		}
		if cap, ok := vehicle["capacity"].([]any); !ok || cap[0].(float64) != 4 {
			t.Errorf("capacity = %v, want [4]", vehicle["capacity"])
		}
		if skills, ok := vehicle["skills"].([]any); !ok || skills[0].(float64) != 1 {
			t.Errorf("skills = %v, want [1]", vehicle["skills"])
		}
		brk := vehicle["breaks"].([]any)[0].(map[string]any)
		if brk["id"].(float64) != 1 || brk["service"].(float64) != 1800 {
			t.Errorf("break = %v, want {id:1 service:1800}", brk)
		}
		if mls, ok := brk["max_load"].([]any); !ok || mls[0].(float64) != 2 {
			t.Errorf("break max_load = %v, want [2]", brk["max_load"])
		}
		costs := vehicle["costs"].(map[string]any)
		if costs["fixed"].(float64) != 10 || costs["per_hour"].(float64) != 2000 ||
			costs["per_task_hour"].(float64) != 300 || costs["per_km"].(float64) != 2 {
			t.Errorf("costs = %v, want {10 2000 300 2}", costs)
		}
		steps := vehicle["steps"].([]any)[0].(map[string]any)
		if steps["type"] != "job" || steps["id"].(float64) != 1 {
			t.Errorf("steps = %v, want [{type:job id:1}]", steps)
		}

		job := (*body)["jobs"].([]any)[0].(map[string]any)
		if job["setup"].(float64) != 60 || job["service"].(float64) != 300 {
			t.Errorf("job setup/service = %v/%v, want 60/300", job["setup"], job["service"])
		}
		if spt, ok := job["setup_per_type"].(map[string]any); !ok || spt["van"].(float64) != 120 {
			t.Errorf("setup_per_type = %v, want {van:120}", job["setup_per_type"])
		}
		if spt, ok := job["service_per_type"].(map[string]any); !ok || spt["van"].(float64) != 600 {
			t.Errorf("service_per_type = %v, want {van:600}", job["service_per_type"])
		}
		if del, ok := job["delivery"].([]any); !ok || del[0].(float64) != 1 {
			t.Errorf("delivery = %v, want [1]", job["delivery"])
		}
		if job["priority"].(float64) != 50 {
			t.Errorf("priority = %v, want 50", job["priority"])
		}
		if tws, ok := job["time_windows"].([]any); !ok || tws[0].([]any)[0].(float64) != 0 {
			t.Errorf("time_windows = %v, want [[0, 14400]]", job["time_windows"])
		}

		shipment := (*body)["shipments"].([]any)[0].(map[string]any)
		pickup := shipment["pickup"].(map[string]any)
		if pickup["id"].(float64) != 2 || pickup["service"].(float64) != 150 {
			t.Errorf("pickup = %v, want {id:2 service:150}", pickup)
		}
		delivery := shipment["delivery"].(map[string]any)
		if delivery["id"].(float64) != 3 {
			t.Errorf("delivery id = %v, want 3", delivery["id"])
		}
		if amt, ok := shipment["amount"].([]any); !ok || amt[0].(float64) != 2 {
			t.Errorf("amount = %v, want [2]", shipment["amount"])
		}
		if shipment["priority"].(float64) != 75 {
			t.Errorf("shipment priority = %v, want 75", shipment["priority"])
		}

		matrices := (*body)["matrices"].(map[string]any)
		car := matrices["car"].(map[string]any)
		if durs, ok := car["durations"].([]any); !ok || len(durs) != 2 {
			t.Errorf("car durations = %v, want a 2x2 matrix", car["durations"])
		}
		if _, ok := car["distances"]; !ok {
			t.Error("car distances should be present")
		}

		if options, ok := (*body)["options"].(map[string]any); !ok || options["g"] != true {
			t.Errorf("options = %v, want {g:true}", (*body)["options"])
		}
	})

	t.Run("shipments only", func(t *testing.T) {
		client, body := newBodyCapturingClient(t)

		_, _, err := getOptimize(context.Background(), client, OptimizeInput{
			Vehicles: []OptimizeVehicle{
				{ID: 1, Start: "103.8198,1.3521"},
			},
			Shipments: []OptimizeShipment{
				{
					Pickup:   OptimizeShipmentStep{ID: 1, Location: "103.85,1.29"},
					Delivery: OptimizeShipmentStep{ID: 2, Location: "103.9,1.31"},
				},
			},
		})
		if err != nil {
			t.Fatalf("getOptimize: %v", err)
		}

		if _, ok := (*body)["jobs"]; ok {
			t.Error("jobs should be omitted when only shipments are given")
		}
		if shipments, ok := (*body)["shipments"].([]any); !ok || len(shipments) != 1 {
			t.Errorf("shipments = %v, want one entry", (*body)["shipments"])
		}
	})
}

func TestGetOptimizeInputValidation(t *testing.T) {
	validVehicle := func() OptimizeVehicle {
		return OptimizeVehicle{ID: 1, Start: "103.8198,1.3521"}
	}
	validJob := func() OptimizeJob {
		return OptimizeJob{ID: 1, Location: "103.8514,1.2897"}
	}

	tests := []struct {
		name      string
		input     OptimizeInput
		wantError string
	}{
		{"no vehicles", OptimizeInput{Jobs: []OptimizeJob{validJob()}}, "at least 1 vehicle"},
		{
			"no jobs or shipments",
			OptimizeInput{Vehicles: []OptimizeVehicle{validVehicle()}},
			"at least 1 job or shipment",
		},
		{
			"vehicle without start or end",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{{ID: 1}},
				Jobs:     []OptimizeJob{validJob()},
			},
			"vehicles[0]",
		},
		{
			"malformed vehicle start reports its position",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{{ID: 1, Start: "not-a-coordinate"}},
				Jobs:     []OptimizeJob{validJob()},
			},
			"vehicles[0].start",
		},
		{
			"malformed job location reports its position",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{validVehicle()},
				Jobs: []OptimizeJob{
					validJob(),
					{ID: 2, Location: "not-a-coordinate"},
				},
			},
			"jobs[1].location",
		},
		{
			"unsupported profile",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{{ID: 1, Profile: "walking", Start: "103.8198,1.3521"}},
				Jobs:     []OptimizeJob{validJob()},
			},
			"vehicles[0].profile",
		},
		{
			"dropped car synonym",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{{ID: 1, Profile: "car", Start: "103.8198,1.3521"}},
				Jobs:     []OptimizeJob{validJob()},
			},
			"vehicles[0].profile",
		},
		{
			"duplicate vehicle ids",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{
					validVehicle(),
					{ID: 1, Start: "103.84,1.30"},
				},
				Jobs: []OptimizeJob{validJob()},
			},
			"vehicle ids must be unique",
		},
		{
			"duplicate job ids",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{validVehicle()},
				Jobs: []OptimizeJob{
					validJob(),
					{ID: 1, Location: "103.84,1.30"},
				},
			},
			"job ids must be unique",
		},
		{
			"job without location or index",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{validVehicle()},
				Jobs:     []OptimizeJob{{ID: 1}},
			},
			"jobs[0]: a location or a location_index",
		},
		{
			"negative location index",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{validVehicle()},
				Jobs: []OptimizeJob{{
					ID: 1, Location: "103.8514,1.2897", LocationIndex: intPtr(-1),
				}},
			},
			"jobs[0].location_index",
		},
		{
			"priority above 100",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{validVehicle()},
				Jobs:     []OptimizeJob{{ID: 1, Location: "103.8514,1.2897", Priority: 150}},
			},
			"jobs[0].priority",
		},
		{
			"negative priority",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{validVehicle()},
				Jobs:     []OptimizeJob{{ID: 1, Location: "103.8514,1.2897", Priority: -1}},
			},
			"must be between 0 and 100",
		},
		{
			"reversed job time window",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{validVehicle()},
				Jobs: []OptimizeJob{{
					ID: 1, Location: "103.8514,1.2897",
					TimeWindows: []OptimizeTimeWindow{{14400, 0}},
				}},
			},
			"jobs[0].time_windows[0]",
		},
		{
			"reversed vehicle time window",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{{
					ID: 1, Start: "103.8198,1.3521", TimeWindow: &OptimizeTimeWindow{14400, 0},
				}},
				Jobs: []OptimizeJob{validJob()},
			},
			"vehicles[0].time_window",
		},
		{
			"speed factor above 5",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{{ID: 1, Start: "103.8198,1.3521", SpeedFactor: 6}},
				Jobs:     []OptimizeJob{validJob()},
			},
			"vehicles[0].speed_factor",
		},
		{
			"break id zero",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{{
					ID: 1, Start: "103.8198,1.3521", Breaks: []OptimizeBreak{{ID: 0}},
				}},
				Jobs: []OptimizeJob{validJob()},
			},
			"vehicles[0].breaks[0].id",
		},
		{
			"duplicate break ids",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{{
					ID:     1,
					Start:  "103.8198,1.3521",
					Breaks: []OptimizeBreak{{ID: 1}, {ID: 1}},
				}},
				Jobs: []OptimizeJob{validJob()},
			},
			"break ids must be unique",
		},
		{
			"negative job service",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{validVehicle()},
				Jobs:     []OptimizeJob{{ID: 1, Location: "103.8514,1.2897", Service: -1}},
			},
			"jobs[0].service",
		},
		{
			"negative capacity entry",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{{ID: 1, Start: "103.8198,1.3521", Capacity: []int{4, -1}}},
				Jobs:     []OptimizeJob{validJob()},
			},
			"vehicles[0].capacity",
		},
		{
			"amount length mismatch",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{{ID: 1, Start: "103.8198,1.3521", Capacity: []int{4, 2}}},
				Jobs: []OptimizeJob{{
					ID: 1, Location: "103.8514,1.2897", Delivery: []int{1},
				}},
			},
			"does not match vehicles[0].capacity",
		},
		{
			"duplicate pickup ids",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{validVehicle()},
				Shipments: []OptimizeShipment{
					{
						Pickup:   OptimizeShipmentStep{ID: 1, Location: "103.85,1.29"},
						Delivery: OptimizeShipmentStep{ID: 2, Location: "103.9,1.31"},
					},
					{
						Pickup:   OptimizeShipmentStep{ID: 1, Location: "103.86,1.30"},
						Delivery: OptimizeShipmentStep{ID: 3, Location: "103.91,1.32"},
					},
				},
			},
			"pickup ids must be unique",
		},
		{
			"duplicate delivery ids",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{validVehicle()},
				Shipments: []OptimizeShipment{
					{
						Pickup:   OptimizeShipmentStep{ID: 1, Location: "103.85,1.29"},
						Delivery: OptimizeShipmentStep{ID: 2, Location: "103.9,1.31"},
					},
					{
						Pickup:   OptimizeShipmentStep{ID: 3, Location: "103.86,1.30"},
						Delivery: OptimizeShipmentStep{ID: 2, Location: "103.91,1.32"},
					},
				},
			},
			"delivery ids must be unique",
		},
		{
			"invalid matrices key",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{validVehicle()},
				Jobs:     []OptimizeJob{validJob()},
				Matrices: map[string]OptimizeMatrix{
					"truck": {Durations: [][]int{{0, 1}, {1, 0}}},
				},
			},
			"matrices key",
		},
		{
			"invalid vehicle step type",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{{
					ID: 1, Start: "103.8198,1.3521",
					Steps: []OptimizeVehicleStep{{Type: "start", ID: 1}},
				}},
				Jobs: []OptimizeJob{validJob()},
			},
			"vehicles[0].steps[0].type",
		},
		{
			"malformed shipment pickup location",
			OptimizeInput{
				Vehicles: []OptimizeVehicle{validVehicle()},
				Shipments: []OptimizeShipment{
					{
						Pickup:   OptimizeShipmentStep{ID: 1, Location: "not-a-coordinate"},
						Delivery: OptimizeShipmentStep{ID: 2, Location: "103.9,1.31"},
					},
				},
			},
			"shipments[0].pickup.location",
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
		{DrivingProfile, "car"},
		{"", "car"},
		{MotorcycleProfile, "motorcycle"},
	}

	for _, tt := range tests {
		t.Run(string(tt.profile), func(t *testing.T) {
			if got := optimizeProfile(tt.profile); got != tt.want {
				t.Errorf("optimizeProfile(%q) = %q, want %q", tt.profile, got, tt.want)
			}
		})
	}
}

// intPtr returns a pointer to v, for optional int input fields.
func intPtr(v int) *int { return &v }

// Shipment step ids are unique per kind — a pickup and a delivery may share
// the same numeric id, matching the engine's rule.
func TestOptimizeShipmentIdsArePerKind(t *testing.T) {
	client := newTestClient(t, jsonHandler(http.StatusOK, optimizeEmptySolution))

	_, _, err := getOptimize(context.Background(), client, OptimizeInput{
		Vehicles: []OptimizeVehicle{
			{ID: 1, Start: "103.8198,1.3521"},
		},
		Shipments: []OptimizeShipment{
			{
				Pickup:   OptimizeShipmentStep{ID: 1, Location: "103.85,1.29"},
				Delivery: OptimizeShipmentStep{ID: 1, Location: "103.9,1.31"},
			},
		},
	})
	if err != nil {
		t.Fatalf("getOptimize: %v", err)
	}
}
