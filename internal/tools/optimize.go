package tools

import (
	"context"
	"fmt"
	"strings"

	justrouting "github.com/justrouting/go-client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type OptimizeConfig struct {
	APIKey string
}

type OptimizeInput struct {
	Vehicles []OptimizeVehicleInput `json:"vehicles" jsonschema:"the available fleet; at least 1 vehicle required; each vehicle needs a start or an end"`
	Jobs     []OptimizeJobInput     `json:"jobs" jsonschema:"the tasks to assign; at least 1 job required"`
}

type OptimizeVehicleInput struct {
	ID      int    `json:"id" jsonschema:"unique vehicle identifier, for example 1"`
	Profile string `json:"profile,omitempty" jsonschema:"routing profile for this vehicle: set to \"motorcycle\" when the user's request mentions a motorcycle or motorbike; otherwise omit it or set it to \"car\" for the default driving profile"`
	Start   string `json:"start,omitempty" jsonschema:"vehicle start location in longitude,latitude format, for example \"103.8198,1.3521\"; for a round trip set it to the vehicle's current location; omit or pass an empty string when the vehicle may start anywhere"`
	End     string `json:"end,omitempty" jsonschema:"vehicle end location in longitude,latitude format, for example \"103.8198,1.3521\"; for a round trip set it to the vehicle's current location; omit or pass an empty string when the vehicle may end anywhere"`
}

type OptimizeJobInput struct {
	ID       int    `json:"id" jsonschema:"unique job identifier, for example 1"`
	Location string `json:"location" jsonschema:"job location in longitude,latitude format, for example \"103.8514,1.2897\""`
}

type OptimizeOutput struct {
	Summary    OptimizeSummaryOutput      `json:"summary" jsonschema:"aggregate cost and time across all routes"`
	Routes     []OptimizeRouteOutput      `json:"routes" jsonschema:"one itinerary per vehicle that was used"`
	Unassigned []OptimizeUnassignedOutput `json:"unassigned" jsonschema:"jobs that no vehicle could serve"`
}

type OptimizeSummaryOutput struct {
	Cost        int `json:"cost" jsonschema:"total cost of the solution"`
	Routes      int `json:"routes" jsonschema:"number of vehicles used"`
	Unassigned  int `json:"unassigned" jsonschema:"number of jobs no vehicle could serve"`
	Setup       int `json:"setup" jsonschema:"total setup time in seconds"`
	Service     int `json:"service" jsonschema:"total on-site service time in seconds"`
	Duration    int `json:"duration" jsonschema:"total route duration in seconds"`
	WaitingTime int `json:"waiting_time" jsonschema:"total waiting time in seconds"`
	Priority    int `json:"priority" jsonschema:"total priority sum"`
	Distance    int `json:"distance,omitempty" jsonschema:"total distance in meters"`
}

type OptimizeRouteOutput struct {
	Vehicle     int                  `json:"vehicle" jsonschema:"vehicle id serving this route"`
	Cost        int                  `json:"cost" jsonschema:"route cost"`
	Setup       int                  `json:"setup" jsonschema:"route setup time in seconds"`
	Service     int                  `json:"service" jsonschema:"route on-site service time in seconds"`
	Duration    int                  `json:"duration" jsonschema:"route duration in seconds"`
	WaitingTime int                  `json:"waiting_time" jsonschema:"route waiting time in seconds"`
	Priority    int                  `json:"priority" jsonschema:"route priority sum"`
	Distance    int                  `json:"distance,omitempty" jsonschema:"route distance in meters"`
	Steps       []OptimizeStepOutput `json:"steps" jsonschema:"stops in visiting order, from the vehicle's start to its end"`
}

type OptimizeStepOutput struct {
	Type        string            `json:"type" jsonschema:"stop type: \"start\", \"job\", or \"end\""`
	Location    justrouting.Point `json:"location,omitempty" jsonschema:"stop location as [longitude, latitude]"`
	Job         int               `json:"job,omitempty" jsonschema:"job id, only on \"job\" steps"`
	Setup       int               `json:"setup,omitempty" jsonschema:"setup time in seconds"`
	Service     int               `json:"service,omitempty" jsonschema:"on-site service time in seconds"`
	WaitingTime int               `json:"waiting_time,omitempty" jsonschema:"waiting time before this stop in seconds"`
	Arrival     int               `json:"arrival" jsonschema:"arrival time at this stop in seconds"`
	Duration    int               `json:"duration" jsonschema:"travel duration to this stop in seconds"`
	Distance    int               `json:"distance,omitempty" jsonschema:"travel distance to this stop in meters"`
	Description string            `json:"description,omitempty" jsonschema:"label echoed from the request"`
}

type OptimizeUnassignedOutput struct {
	ID          int               `json:"id" jsonschema:"id of the job that could not be served"`
	Type        string            `json:"type,omitempty" jsonschema:"task type, always \"job\" for now"`
	Location    justrouting.Point `json:"location,omitempty" jsonschema:"job location as [longitude, latitude]"`
	Description string            `json:"description,omitempty" jsonschema:"label echoed from the request"`
}

func RegisterOptimizeTool(
	server *mcp.Server,
	cfg OptimizeConfig,
) {
	client := justrouting.NewClient(cfg.APIKey)

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name: "optimize",
			Description: `
Solve a vehicle routing problem using JustRouting: assign jobs to vehicles and order each vehicle's stops.

The input is a fleet of vehicles and a list of jobs:
- Each vehicle has an "id", an optional "profile", and "start"/"end" locations in
  longitude,latitude format. For a round trip (the vehicle returns to where it began),
  set both start and end to the vehicle's current location. When a vehicle may start
  or end anywhere, omit the field or pass an empty string.
- Each job has an "id" and a "location" in longitude,latitude format.

If the user gives place names or addresses instead of coordinates, do not
guess coordinates. Call the geocode tool first for every place, then pass
each geocode call's first (best) result "coordinates" value into this tool.

Coordinates must use longitude,latitude format.
For example: 103.8198,1.3521

Optional per-vehicle "profile" input selects the routing profile:
- If the user's request mentions a motorcycle or motorbike, set profile to "motorcycle".
- Otherwise (the user asks to drive, or no vehicle is mentioned), omit profile or set it to "car" to get the default driving profile.

The response contains:
- "routes": one itinerary per vehicle that was used. Read each route's "steps" in
  order — "job" steps carry the "job" id plus "arrival" and "duration" — to tell
  which vehicle serves which jobs and when.
- "unassigned": the jobs no vehicle could serve.
- "summary": aggregate cost, duration and distance across all routes.
			`,
		},
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			input OptimizeInput,
		) (*mcp.CallToolResult, OptimizeOutput, error) {
			return getOptimize(ctx, client, input)
		},
	)
}

func getOptimize(
	ctx context.Context,
	client *justrouting.Client,
	input OptimizeInput,
) (*mcp.CallToolResult, OptimizeOutput, error) {
	req, err := buildOptimizeRequest(input)
	if err != nil {
		return nil, OptimizeOutput{}, err
	}

	solution, err := client.Optimization.Solve(ctx, req)
	if err != nil {
		return nil, OptimizeOutput{}, fmt.Errorf("optimization failed: %w", err)
	}

	return nil, buildOptimizeOutput(solution), nil
}

// buildOptimizeRequest converts the MCP input into a go-client request. It
// validates everything that yields a clearer message here than the client's
// own checks would — coordinate parsing, profile names, id uniqueness, and
// the start-or-end rule — so the LLM gets an actionable error without
// spending a request.
func buildOptimizeRequest(input OptimizeInput) (*justrouting.OptimizationRequest, error) {
	if len(input.Vehicles) == 0 {
		return nil, fmt.Errorf("at least 1 vehicle is required, got 0")
	}
	if len(input.Jobs) == 0 {
		return nil, fmt.Errorf("at least 1 job is required, got 0")
	}

	vehicles := make([]justrouting.Vehicle, len(input.Vehicles))
	seenVehicles := make(map[int]bool, len(input.Vehicles))
	for i, v := range input.Vehicles {
		if v.ID <= 0 {
			return nil, fmt.Errorf("invalid vehicles[%d].id = %d: must be a positive integer", i, v.ID)
		}
		if seenVehicles[v.ID] {
			return nil, fmt.Errorf("invalid vehicles[%d].id = %d: vehicle ids must be unique", i, v.ID)
		}
		seenVehicles[v.ID] = true

		profile, err := normalizeVehicleProfile(v.Profile)
		if err != nil {
			return nil, fmt.Errorf("invalid vehicles[%d].profile: %w", i, err)
		}

		vehicles[i] = justrouting.Vehicle{ID: v.ID, Profile: profile}

		// An empty start/end means the vehicle may start or end anywhere:
		// leave the point nil so the field is omitted from the request
		// body. The go-client requires a start or an end per vehicle; that
		// rule is checked below with the vehicle's index named.
		if strings.TrimSpace(v.Start) != "" {
			start, err := parsePoint(v.Start)
			if err != nil {
				return nil, fmt.Errorf("invalid vehicles[%d].start: %w", i, err)
			}
			vehicles[i].Start = start
		}
		if strings.TrimSpace(v.End) != "" {
			end, err := parsePoint(v.End)
			if err != nil {
				return nil, fmt.Errorf("invalid vehicles[%d].end: %w", i, err)
			}
			vehicles[i].End = end
		}

		if len(vehicles[i].Start) == 0 && len(vehicles[i].End) == 0 {
			return nil, fmt.Errorf(
				"invalid vehicles[%d]: a start or an end location is required (both may not be empty)",
				i,
			)
		}
	}

	jobs := make([]justrouting.Job, len(input.Jobs))
	seenJobs := make(map[int]bool, len(input.Jobs))
	for i, j := range input.Jobs {
		if j.ID <= 0 {
			return nil, fmt.Errorf("invalid jobs[%d].id = %d: must be a positive integer", i, j.ID)
		}
		if seenJobs[j.ID] {
			return nil, fmt.Errorf("invalid jobs[%d].id = %d: job ids must be unique", i, j.ID)
		}
		seenJobs[j.ID] = true

		location, err := parsePoint(j.Location)
		if err != nil {
			return nil, fmt.Errorf("invalid jobs[%d].location: %w", i, err)
		}
		jobs[i] = justrouting.Job{ID: j.ID, Location: location}
	}

	return &justrouting.OptimizationRequest{
		Vehicles: vehicles,
		Jobs:     jobs,
	}, nil
}

// normalizeVehicleProfile maps user-facing profile names to the optimization
// engine's profiles, which differ from the route/table API: an empty profile
// (field omitted) and car synonyms map to "car", the engine's default.
// Motorcycle synonyms map to "motorcycle". Any other value is rejected so
// unsupported profiles fail fast with a clear error.
func normalizeVehicleProfile(profile string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "", "car", "driving":
		return "car", nil
	case "motorcycle", "motorbike":
		return "motorcycle", nil
	default:
		return "", fmt.Errorf(
			"unsupported profile %q: must be one of \"car\", \"driving\", \"motorcycle\", \"motorbike\"",
			profile,
		)
	}
}

// buildOptimizeOutput maps the client solution onto the tool's own output
// structs, dropping the fields the minimal input can never populate
// (delivery, pickup, load, geometry). The mapping is nil-safe: the engine
// may return nil routes, steps or unassigned entries.
func buildOptimizeOutput(s *justrouting.Solution) OptimizeOutput {
	out := OptimizeOutput{
		Summary: OptimizeSummaryOutput{
			Cost:        s.Summary.Cost,
			Routes:      s.Summary.Routes,
			Unassigned:  s.Summary.Unassigned,
			Setup:       s.Summary.Setup,
			Service:     s.Summary.Service,
			Duration:    s.Summary.Duration,
			WaitingTime: s.Summary.WaitingTime,
			Priority:    s.Summary.Priority,
			Distance:    s.Summary.Distance,
		},
	}

	out.Routes = make([]OptimizeRouteOutput, 0, len(s.Routes))
	for _, r := range s.Routes {
		if r == nil {
			continue
		}
		route := OptimizeRouteOutput{
			Vehicle:     r.Vehicle,
			Cost:        r.Cost,
			Setup:       r.Setup,
			Service:     r.Service,
			Duration:    r.Duration,
			WaitingTime: r.WaitingTime,
			Priority:    r.Priority,
			Distance:    r.Distance,
		}
		route.Steps = make([]OptimizeStepOutput, 0, len(r.Steps))
		for _, st := range r.Steps {
			if st == nil {
				continue
			}
			route.Steps = append(route.Steps, OptimizeStepOutput{
				Type:        st.Type,
				Location:    st.Location,
				Job:         st.Job,
				Setup:       st.Setup,
				Service:     st.Service,
				WaitingTime: st.WaitingTime,
				Arrival:     st.Arrival,
				Duration:    st.Duration,
				Distance:    st.Distance,
				Description: st.Description,
			})
		}
		out.Routes = append(out.Routes, route)
	}

	out.Unassigned = make([]OptimizeUnassignedOutput, 0, len(s.Unassigned))
	for _, u := range s.Unassigned {
		if u == nil {
			continue
		}
		out.Unassigned = append(out.Unassigned, OptimizeUnassignedOutput{
			ID:          u.ID,
			Type:        u.Type,
			Location:    u.Location,
			Description: u.Description,
		})
	}

	return out
}
