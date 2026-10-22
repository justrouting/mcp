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

// OptimizeTimeWindow is an inclusive [start, end] pair in seconds. Use one
// clock for the whole request: either relative to the planning horizon start
// or absolute UNIX timestamps.
type OptimizeTimeWindow [2]int

type OptimizeInput struct {
	Vehicles  []OptimizeVehicle         `json:"vehicles" jsonschema:"the available fleet; at least 1 vehicle required; each vehicle needs a start (or start_index) or an end (or end_index)"`
	Jobs      []OptimizeJob             `json:"jobs,omitempty" jsonschema:"single-location tasks to assign; at least 1 job or shipment is required"`
	Shipments []OptimizeShipment        `json:"shipments,omitempty" jsonschema:"pickup-and-delivery tasks; the same vehicle must serve the pickup before the delivery; at least 1 job or shipment is required"`
	Matrices  map[string]OptimizeMatrix `json:"matrices,omitempty" jsonschema:"custom matrices per routing profile; keys must be \"car\" or \"motorcycle\"; only needed when the user supplies their own travel times"`
	Options   *OptimizeOptions          `json:"options,omitempty" jsonschema:"solver options, for example {\"g\": true} to include route geometry in the result"`
}

type OptimizeVehicle struct {
	ID            int                   `json:"id" jsonschema:"unique vehicle identifier, for example 1"`
	Profile       Profile               `json:"profile,omitempty" jsonschema:"routing profile for this vehicle: set to \"motorcycle\" when the user's request mentions a motorcycle or motorbike; otherwise omit it for the default driving profile"`
	Description   string                `json:"description,omitempty" jsonschema:"label echoed back in the result, for example \"van A\"; helps you match result routes to the vehicles the user described"`
	Start         string                `json:"start,omitempty" jsonschema:"vehicle start location in longitude,latitude format, for example \"103.8198,1.3521\"; for a round trip set it to the vehicle's current location; omit or pass an empty string when the vehicle may start anywhere"`
	StartIndex    *int                  `json:"start_index,omitempty" jsonschema:"row or column of this vehicle's start in custom matrices; use instead of start when matrices are provided"`
	End           string                `json:"end,omitempty" jsonschema:"vehicle end location in longitude,latitude format, for example \"103.8198,1.3521\"; for a round trip set it to the vehicle's current location; omit or pass an empty string when the vehicle may end anywhere"`
	EndIndex      *int                  `json:"end_index,omitempty" jsonschema:"row or column of this vehicle's end in custom matrices; use instead of end when matrices are provided"`
	Capacity      []int                 `json:"capacity,omitempty" jsonschema:"maximum load in each dimension, for example [1000, 4] for 1000 kilograms and 4 pallets; every delivery, pickup and amount vector in the request must use this same length"`
	Skills        []int                 `json:"skills,omitempty" jsonschema:"capabilities this vehicle provides, for example [1, 2]; a task is only served by a vehicle that has all the task's skills"`
	Type          string                `json:"type,omitempty" jsonschema:"vehicle type name, for example \"van\"; lets jobs and shipments override setup and service times per vehicle type"`
	TimeWindow    *OptimizeTimeWindow   `json:"time_window,omitempty" jsonschema:"when the vehicle is available, as [start, end] in seconds, both ends inclusive"`
	Breaks        []OptimizeBreak       `json:"breaks,omitempty" jsonschema:"rest periods during the route, each with an id unique within the vehicle"`
	SpeedFactor   float64               `json:"speed_factor,omitempty" jsonschema:"multiplier for this vehicle's travel times, in the range (0, 5], for example 0.8 for a slow truck; omit for the default 1"`
	MaxTasks      int                   `json:"max_tasks,omitempty" jsonschema:"maximum number of tasks this vehicle may serve"`
	MaxTravelTime int                   `json:"max_travel_time,omitempty" jsonschema:"maximum total travel time in seconds for this vehicle"`
	MaxDistance   int                   `json:"max_distance,omitempty" jsonschema:"maximum total travel distance in meters for this vehicle"`
	Costs         *OptimizeVehicleCosts `json:"costs,omitempty" jsonschema:"cost factors used to value this vehicle; omit for the engine defaults"`
	Steps         []OptimizeVehicleStep `json:"steps,omitempty" jsonschema:"custom route that warm-starts the search; each step references a task by type and id; only \"job\", \"pickup\" and \"delivery\" steps are used"`
}

type OptimizeBreak struct {
	ID          int                  `json:"id" jsonschema:"unique break identifier within the vehicle, for example 1"`
	TimeWindows []OptimizeTimeWindow `json:"time_windows,omitempty" jsonschema:"valid slots for the break to start, each as [start, end] in seconds"`
	Service     int                  `json:"service,omitempty" jsonschema:"break duration in seconds, default 0"`
	Description string               `json:"description,omitempty" jsonschema:"label echoed back in the result"`
	MaxLoad     []int                `json:"max_load,omitempty" jsonschema:"maximum vehicle load in each dimension for this break to happen"`
}

type OptimizeVehicleCosts struct {
	Fixed       int `json:"fixed,omitempty" jsonschema:"one-time cost of using this vehicle, default 0"`
	PerHour     int `json:"per_hour,omitempty" jsonschema:"cost of one hour of travel time, default 3600"`
	PerTaskHour int `json:"per_task_hour,omitempty" jsonschema:"cost of one hour of task time (setup plus service), default 0"`
	PerKM       int `json:"per_km,omitempty" jsonschema:"cost of one kilometer of travel, default 0"`
}

type OptimizeVehicleStep struct {
	Type string `json:"type" jsonschema:"task type of this step: \"job\", \"pickup\", or \"delivery\""`
	ID   int    `json:"id" jsonschema:"id of the job or shipment step at this position, for example 3"`
}

type OptimizeJob struct {
	ID             int                  `json:"id" jsonschema:"unique job identifier, for example 1"`
	Description    string               `json:"description,omitempty" jsonschema:"label echoed back in the result; helps you match result steps to the tasks the user described"`
	Location       string               `json:"location,omitempty" jsonschema:"job location in longitude,latitude format, for example \"103.8514,1.2897\"; omit when custom matrices cover every profile in use (then location_index is required)"`
	LocationIndex  *int                 `json:"location_index,omitempty" jsonschema:"row or column of this job in custom matrices; use instead of location when matrices are provided"`
	Setup          int                  `json:"setup,omitempty" jsonschema:"fixed preparation time in seconds before service, default 0"`
	Service        int                  `json:"service,omitempty" jsonschema:"time spent on site in seconds, default 0"`
	SetupPerType   map[string]int       `json:"setup_per_type,omitempty" jsonschema:"setup time in seconds per vehicle type, for example {\"van\": 300}; overrides setup for vehicles of that type"`
	ServicePerType map[string]int       `json:"service_per_type,omitempty" jsonschema:"service time in seconds per vehicle type, for example {\"van\": 600}; overrides service for vehicles of that type"`
	Delivery       []int                `json:"delivery,omitempty" jsonschema:"amounts unloaded at this job, one value per capacity dimension, for example [50]; the vehicle starts the route with all deliveries loaded"`
	Pickup         []int                `json:"pickup,omitempty" jsonschema:"amounts loaded at this job, one value per capacity dimension, for example [50]; the vehicle returns the pickups at the end"`
	Skills         []int                `json:"skills,omitempty" jsonschema:"capabilities a vehicle must have to serve this job; the vehicle must have all of them"`
	Priority       int                  `json:"priority,omitempty" jsonschema:"rank from 0 to 100 used only when not all tasks fit; higher priority tasks are preferred, default 0"`
	TimeWindows    []OptimizeTimeWindow `json:"time_windows,omitempty" jsonschema:"valid slots for service to start, each as [start, end] in seconds; the vehicle waits before a window that has not opened yet"`
}

type OptimizeShipment struct {
	Pickup   OptimizeShipmentStep `json:"pickup" jsonschema:"the pickup step, served before the delivery by the same vehicle"`
	Delivery OptimizeShipmentStep `json:"delivery" jsonschema:"the delivery step, served after the pickup by the same vehicle"`
	Amount   []int                `json:"amount,omitempty" jsonschema:"amounts carried between pickup and delivery, one value per capacity dimension, for example [50]"`
	Skills   []int                `json:"skills,omitempty" jsonschema:"capabilities a vehicle must have to serve this shipment; the vehicle must have all of them"`
	Priority int                  `json:"priority,omitempty" jsonschema:"rank from 0 to 100 used only when not all tasks fit, default 0"`
}

type OptimizeShipmentStep struct {
	ID             int                  `json:"id" jsonschema:"unique step identifier, for example 1; pickup ids must be unique among pickups and delivery ids among deliveries"`
	Description    string               `json:"description,omitempty" jsonschema:"label echoed back in the result"`
	Location       string               `json:"location,omitempty" jsonschema:"step location in longitude,latitude format, for example \"103.8514,1.2897\"; omit when custom matrices cover every profile in use (then location_index is required)"`
	LocationIndex  *int                 `json:"location_index,omitempty" jsonschema:"row or column of this step in custom matrices; use instead of location when matrices are provided"`
	Setup          int                  `json:"setup,omitempty" jsonschema:"fixed preparation time in seconds before service, default 0"`
	Service        int                  `json:"service,omitempty" jsonschema:"time spent at this step in seconds, default 0"`
	SetupPerType   map[string]int       `json:"setup_per_type,omitempty" jsonschema:"setup time in seconds per vehicle type, for example {\"van\": 300}; overrides setup for vehicles of that type"`
	ServicePerType map[string]int       `json:"service_per_type,omitempty" jsonschema:"service time in seconds per vehicle type, for example {\"van\": 600}; overrides service for vehicles of that type"`
	TimeWindows    []OptimizeTimeWindow `json:"time_windows,omitempty" jsonschema:"valid slots for service to start, each as [start, end] in seconds"`
}

type OptimizeMatrix struct {
	Durations [][]int `json:"durations,omitempty" jsonschema:"travel-time matrix in seconds; row is origin, column is destination; providing durations for every profile in use makes location, start and end optional (use the *_index fields instead)"`
	Distances [][]int `json:"distances,omitempty" jsonschema:"distance matrix in meters; row is origin, column is destination; requires durations"`
	Costs     [][]int `json:"costs,omitempty" jsonschema:"cost matrix used for route cost evaluation; row is origin, column is destination"`
}

type OptimizeOptions struct {
	Geometry bool `json:"g" jsonschema:"when true, each route in the result includes a road-following encoded polyline and travel distances; costs extra computation, so only set it when the user asks for a route shape or map display"`
}

type OptimizeOutput struct {
	Summary    OptimizeSummary      `json:"summary" jsonschema:"aggregate cost and time across all routes"`
	Routes     []OptimizeRoute      `json:"routes" jsonschema:"one itinerary per vehicle that was used"`
	Unassigned []OptimizeUnassigned `json:"unassigned" jsonschema:"tasks that no vehicle could serve"`
}

type OptimizeSummary struct {
	Cost        int                 `json:"cost" jsonschema:"total cost of the solution"`
	Routes      int                 `json:"routes" jsonschema:"number of vehicles used"`
	Unassigned  int                 `json:"unassigned" jsonschema:"number of tasks no vehicle could serve"`
	Setup       int                 `json:"setup" jsonschema:"total setup time in seconds"`
	Service     int                 `json:"service" jsonschema:"total on-site service time in seconds"`
	Duration    int                 `json:"duration" jsonschema:"total route duration in seconds"`
	WaitingTime int                 `json:"waiting_time" jsonschema:"total waiting time in seconds"`
	Priority    int                 `json:"priority" jsonschema:"total priority sum"`
	Distance    int                 `json:"distance,omitempty" jsonschema:"total distance in meters"`
	Delivery    []int               `json:"delivery,omitempty" jsonschema:"total amounts delivered, one value per capacity dimension"`
	Pickup      []int               `json:"pickup,omitempty" jsonschema:"total amounts picked up, one value per capacity dimension"`
	Violations  []OptimizeViolation `json:"violations,omitempty" jsonschema:"constraint violations; always empty on the hosted API"`
}

type OptimizeRoute struct {
	Vehicle     int                 `json:"vehicle" jsonschema:"vehicle id serving this route"`
	Cost        int                 `json:"cost" jsonschema:"route cost"`
	Setup       int                 `json:"setup" jsonschema:"route setup time in seconds"`
	Service     int                 `json:"service" jsonschema:"route on-site service time in seconds"`
	Duration    int                 `json:"duration" jsonschema:"route duration in seconds"`
	WaitingTime int                 `json:"waiting_time" jsonschema:"route waiting time in seconds"`
	Priority    int                 `json:"priority" jsonschema:"route priority sum"`
	Distance    int                 `json:"distance,omitempty" jsonschema:"route distance in meters"`
	Delivery    []int               `json:"delivery,omitempty" jsonschema:"total amounts delivered on this route, one value per capacity dimension"`
	Pickup      []int               `json:"pickup,omitempty" jsonschema:"total amounts picked up on this route, one value per capacity dimension"`
	Description string              `json:"description,omitempty" jsonschema:"vehicle description echoed from the request"`
	Geometry    string              `json:"geometry,omitempty" jsonschema:"encoded polyline of the route, only when options.g was true"`
	Steps       []OptimizeStep      `json:"steps" jsonschema:"stops in visiting order, from the vehicle's start to its end"`
	Violations  []OptimizeViolation `json:"violations,omitempty" jsonschema:"constraint violations for this route; always empty on the hosted API"`
}

type OptimizeStep struct {
	Type          string              `json:"type" jsonschema:"stop type: \"start\", \"job\", \"pickup\", \"delivery\", \"break\", or \"end\""`
	Location      justrouting.Point   `json:"location,omitempty" jsonschema:"stop location as [longitude, latitude]"`
	LocationIndex int                 `json:"location_index,omitempty" jsonschema:"row/column in custom matrices, when the request used matrices"`
	Job           int                 `json:"job,omitempty" jsonschema:"job id, only on \"job\" steps"`
	ID            int                 `json:"id,omitempty" jsonschema:"task id, on \"job\", \"pickup\", \"delivery\", and \"break\" steps"`
	Setup         int                 `json:"setup,omitempty" jsonschema:"setup time in seconds"`
	Service       int                 `json:"service,omitempty" jsonschema:"on-site service time in seconds"`
	WaitingTime   int                 `json:"waiting_time,omitempty" jsonschema:"waiting time before this stop in seconds"`
	Arrival       int                 `json:"arrival" jsonschema:"arrival time at this stop in seconds"`
	Duration      int                 `json:"duration" jsonschema:"travel duration to this stop in seconds"`
	Distance      int                 `json:"distance,omitempty" jsonschema:"travel distance to this stop in meters"`
	Load          []int               `json:"load,omitempty" jsonschema:"vehicle load after this stop, one value per capacity dimension"`
	Description   string              `json:"description,omitempty" jsonschema:"label echoed from the request"`
	Violations    []OptimizeViolation `json:"violations,omitempty" jsonschema:"constraint violations for this step; always empty on the hosted API"`
}

type OptimizeUnassigned struct {
	ID            int               `json:"id" jsonschema:"id of the task that could not be served"`
	Type          string            `json:"type,omitempty" jsonschema:"task type: \"job\", \"pickup\", or \"delivery\""`
	Location      justrouting.Point `json:"location,omitempty" jsonschema:"task location as [longitude, latitude]"`
	LocationIndex int               `json:"location_index,omitempty" jsonschema:"row/column in custom matrices, when the request used matrices"`
	Description   string            `json:"description,omitempty" jsonschema:"label echoed from the request"`
}

type OptimizeViolation struct {
	Cause    string `json:"cause" jsonschema:"the violated constraint, for example \"delay\", \"skills\", or \"missing_break\""`
	Duration int    `json:"duration" jsonschema:"lateness in seconds for \"delay\", earliness for \"lead_time\""`
}

func RegisterOptimizeTool(
	server *mcp.Server,
	cfg OptimizeConfig,
) {
	client := justrouting.NewClient(cfg.APIKey)

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        "optimize",
			InputSchema: mustSchema[OptimizeInput]("optimize input schema"),
			Description: `
Solve a vehicle routing problem using JustRouting: assign jobs and shipments to
vehicles and determine the order in which each vehicle should visit its stops.

Use this tool when the user needs to:
- assign multiple deliveries, pickups, or visits to one or more vehicles
- plan routes for a fleet with amounts, capacity, skills, or time windows
- pair pickups with deliveries that one vehicle must serve in order
- determine the order in which each vehicle should visit its assigned tasks
- optimize the overall routing solution

Do not use this tool for:
- calculating a single route between two locations; use the route tool
- comparing travel times or distances between multiple locations; use the table tool

The input consists of:
- "vehicles": the available vehicles, each with a unique "id" and optional
  start, end, routing profile, capacity, skills, time window, breaks, costs,
  and limits
- "jobs": single-location tasks to assign, each with a unique "id"
- "shipments": pickup-and-delivery pairs; one vehicle serves the pickup
  before the delivery
- "matrices": optional custom travel-time matrices per profile
- "options": solver options, for example {"g": true} to include route geometry

Locations:
- All locations must use longitude,latitude format.
- For example: "103.8198,1.3521"
- A vehicle may have only a start, only an end, or both.
- For a round trip, set both the vehicle's start and end to the same location.
- If start or end is omitted, the vehicle may start or end anywhere.
- Each vehicle must have at least a start or an end location.

If the user provides place names or addresses instead of coordinates, do not
guess their coordinates. Call the geocode tool first for every location, then
pass each place's best geocoded coordinates to this tool.

Vehicle profiles:
- Each vehicle can use its own routing profile.
- Set profile to "motorcycle" when the user explicitly asks for a motorcycle
  or motorbike route.
- Otherwise, omit profile to use the default driving profile.

Amounts and capacity:
- "capacity" on a vehicle and "delivery", "pickup" and "amount" on tasks are
  multidimensional: one value per dimension, for example [1000, 4] for
  1000 kilograms and 4 pallets.
- All such vectors in one request must share the same length.
- A vehicle serves a set of tasks only if its load never exceeds its
  capacity. Insufficient capacity is not an error: the task is moved to
  "unassigned" instead.

Time windows:
- A time window is an inclusive [start, end] pair in seconds.
- Use one clock for the whole request: either relative (for example
  [0, 14400] means within 4 hours of the planning start) or absolute UNIX
  timestamps.
- Early arrivals wait until the window opens.
- Tasks that cannot be reached in time are moved to "unassigned".

Skills and priorities:
- "skills" are integer capability tags. A task is served only by a vehicle
  that has all the task's skills; a task without skills can be served by any
  vehicle.
- "priority" is 0-100; when not everything fits, higher-priority tasks are
  kept and lower-priority ones are dropped.

Shipments:
- A shipment is a pickup and a delivery served in order by the same vehicle,
  with "amount" loaded at the pickup and unloaded at the delivery.
- Pickup ids must be unique among pickups; delivery ids must be unique among
  deliveries.

Breaks:
- A vehicle's "breaks" are rest periods with an id unique within the vehicle,
  optional time_windows, and a "service" duration in seconds.

Custom matrices:
- "matrices" supplies your own travel-time ("durations"), distance and cost
  matrices per profile ("car" or "motorcycle"). Use it only when the user
  supplies their own travel times; then "location_index", "start_index" and
  "end_index" reference rows and columns instead of coordinates.

Route geometry:
- Set "options": {"g": true} to get an encoded polyline and distances back
  for every route. Use it when the user wants to draw the routes on a map.

The result contains:
- "routes": one itinerary for each vehicle used. Read each route's "steps"
  in order to determine which tasks are assigned to that vehicle and the
  order in which they are visited.
- "unassigned": tasks that could not be served by any vehicle.
- "summary": aggregate cost, duration, waiting time, delivery, pickup, and
  distance across the solution.

A "job" step contains the task id, arrival time, travel duration, and travel
distance for reaching that job. "pickup", "delivery", and "break" steps may
appear between the route's "start" and "end" steps. Use these fields when
explaining the planned schedule to the user.

Do not assume that every task will be assigned. Always check "unassigned"
and mention any tasks that could not be served.
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
// own checks would — coordinate parsing, profile names, id uniqueness,
// amount-vector lengths, time-window order, and the start-or-end rule — so
// the LLM gets an actionable error without spending a request.
func buildOptimizeRequest(input OptimizeInput) (*justrouting.OptimizationRequest, error) {
	if len(input.Vehicles) == 0 {
		return nil, fmt.Errorf("at least 1 vehicle is required, got 0")
	}
	if len(input.Jobs) == 0 && len(input.Shipments) == 0 {
		return nil, fmt.Errorf("at least 1 job or shipment is required, got 0")
	}
	if err := checkAmountLengths(input); err != nil {
		return nil, err
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

		profile, err := normalizeProfile(v.Profile)
		if err != nil {
			return nil, fmt.Errorf("invalid vehicles[%d].profile: %w", i, err)
		}

		vehicles[i] = justrouting.Vehicle{
			ID:          v.ID,
			Profile:     optimizeProfile(profile),
			Description: v.Description,
		}

		// An empty start/end means the vehicle may start or end anywhere:
		// leave the point nil so the field is omitted from the request
		// body. A custom-matrix index counts as an anchor too; the rule is
		// checked below with the vehicle's index named.
		if strings.TrimSpace(v.Start) != "" {
			start, err := parsePoint(v.Start)
			if err != nil {
				return nil, fmt.Errorf("invalid vehicles[%d].start: %w", i, err)
			}
			vehicles[i].Start = start
		}
		if v.StartIndex != nil {
			if *v.StartIndex < 0 {
				return nil, fmt.Errorf("invalid vehicles[%d].start_index = %d: must not be negative", i, *v.StartIndex)
			}
			vehicles[i].StartIndex = v.StartIndex
		}
		if strings.TrimSpace(v.End) != "" {
			end, err := parsePoint(v.End)
			if err != nil {
				return nil, fmt.Errorf("invalid vehicles[%d].end: %w", i, err)
			}
			vehicles[i].End = end
		}
		if v.EndIndex != nil {
			if *v.EndIndex < 0 {
				return nil, fmt.Errorf("invalid vehicles[%d].end_index = %d: must not be negative", i, *v.EndIndex)
			}
			vehicles[i].EndIndex = v.EndIndex
		}

		if len(vehicles[i].Start) == 0 && vehicles[i].StartIndex == nil &&
			len(vehicles[i].End) == 0 && vehicles[i].EndIndex == nil {
			return nil, fmt.Errorf(
				"invalid vehicles[%d]: a start (or start_index) or an end (or end_index) is required",
				i,
			)
		}

		vehicles[i].Capacity = v.Capacity
		if err := checkNonNegative(fmt.Sprintf("vehicles[%d].capacity", i), v.Capacity); err != nil {
			return nil, err
		}
		vehicles[i].Skills = v.Skills
		if err := checkNonNegative(fmt.Sprintf("vehicles[%d].skills", i), v.Skills); err != nil {
			return nil, err
		}
		vehicles[i].Type = v.Type

		if v.TimeWindow != nil {
			if v.TimeWindow[0] > v.TimeWindow[1] {
				return nil, fmt.Errorf(
					"invalid vehicles[%d].time_window = %v: start must be less than or equal to end",
					i, *v.TimeWindow,
				)
			}
			vehicles[i].TimeWindow = (*justrouting.TimeWindow)(v.TimeWindow)
		}

		breaks, err := buildOptimizeBreaks(i, v.Breaks)
		if err != nil {
			return nil, err
		}
		vehicles[i].Breaks = breaks

		if v.SpeedFactor < 0 || v.SpeedFactor > 5 {
			return nil, fmt.Errorf(
				"invalid vehicles[%d].speed_factor = %v: must be in the range (0, 5]",
				i, v.SpeedFactor,
			)
		}
		vehicles[i].SpeedFactor = v.SpeedFactor

		if v.MaxTasks < 0 {
			return nil, fmt.Errorf("invalid vehicles[%d].max_tasks = %d: must not be negative", i, v.MaxTasks)
		}
		if v.MaxTravelTime < 0 {
			return nil, fmt.Errorf("invalid vehicles[%d].max_travel_time = %d: must not be negative", i, v.MaxTravelTime)
		}
		if v.MaxDistance < 0 {
			return nil, fmt.Errorf("invalid vehicles[%d].max_distance = %d: must not be negative", i, v.MaxDistance)
		}
		vehicles[i].MaxTasks = v.MaxTasks
		vehicles[i].MaxTravelTime = v.MaxTravelTime
		vehicles[i].MaxDistance = v.MaxDistance

		if v.Costs != nil {
			vehicles[i].Costs = &justrouting.VehicleCosts{
				Fixed:       v.Costs.Fixed,
				PerHour:     v.Costs.PerHour,
				PerTaskHour: v.Costs.PerTaskHour,
				PerKM:       v.Costs.PerKM,
			}
		}

		steps, err := buildOptimizeVehicleSteps(i, v.Steps)
		if err != nil {
			return nil, err
		}
		vehicles[i].Steps = steps
	}

	jobs := make([]justrouting.Job, len(input.Jobs))
	seenJobs := make(map[int]bool, len(input.Jobs))
	for i, j := range input.Jobs {
		label := fmt.Sprintf("jobs[%d]", i)
		if j.ID <= 0 {
			return nil, fmt.Errorf("invalid jobs[%d].id = %d: must be a positive integer", i, j.ID)
		}
		if seenJobs[j.ID] {
			return nil, fmt.Errorf("invalid jobs[%d].id = %d: job ids must be unique", i, j.ID)
		}
		seenJobs[j.ID] = true

		location, err := buildOptimizeTaskLocation(label, optimizeTaskInput{
			Location:       j.Location,
			LocationIndex:  j.LocationIndex,
			Setup:          j.Setup,
			Service:        j.Service,
			SetupPerType:   j.SetupPerType,
			ServicePerType: j.ServicePerType,
			TimeWindows:    j.TimeWindows,
		})
		if err != nil {
			return nil, err
		}
		if err := checkNonNegative(label+".delivery", j.Delivery); err != nil {
			return nil, err
		}
		if err := checkNonNegative(label+".pickup", j.Pickup); err != nil {
			return nil, err
		}
		if err := checkNonNegative(label+".skills", j.Skills); err != nil {
			return nil, err
		}
		if err := checkPriority(label, j.Priority); err != nil {
			return nil, err
		}

		jobs[i] = justrouting.Job{
			ID:             j.ID,
			Description:    j.Description,
			Location:       location,
			LocationIndex:  j.LocationIndex,
			Setup:          j.Setup,
			Service:        j.Service,
			SetupPerType:   j.SetupPerType,
			ServicePerType: j.ServicePerType,
			Delivery:       j.Delivery,
			Pickup:         j.Pickup,
			Skills:         j.Skills,
			Priority:       j.Priority,
			TimeWindows:    convertTimeWindows(j.TimeWindows),
		}
	}

	shipments := make([]justrouting.Shipment, len(input.Shipments))
	seenPickups := make(map[int]bool, len(input.Shipments))
	seenDeliveries := make(map[int]bool, len(input.Shipments))
	for i, s := range input.Shipments {
		label := fmt.Sprintf("shipments[%d]", i)

		pickup, err := buildOptimizeShipmentStep(label+".pickup", "pickup", s.Pickup, seenPickups)
		if err != nil {
			return nil, err
		}
		delivery, err := buildOptimizeShipmentStep(label+".delivery", "delivery", s.Delivery, seenDeliveries)
		if err != nil {
			return nil, err
		}

		if err := checkNonNegative(label+".amount", s.Amount); err != nil {
			return nil, err
		}
		if err := checkNonNegative(label+".skills", s.Skills); err != nil {
			return nil, err
		}
		if err := checkPriority(label, s.Priority); err != nil {
			return nil, err
		}

		shipments[i] = justrouting.Shipment{
			Pickup:   pickup,
			Delivery: delivery,
			Amount:   s.Amount,
			Skills:   s.Skills,
			Priority: s.Priority,
		}
	}

	matrices := make(map[string]justrouting.CustomMatrix, len(input.Matrices))
	for profile, m := range input.Matrices {
		if profile != "car" && profile != "motorcycle" {
			return nil, fmt.Errorf("invalid matrices key %q: must be \"car\" or \"motorcycle\"", profile)
		}
		if len(m.Durations) == 0 && len(m.Distances) == 0 && len(m.Costs) == 0 {
			return nil, fmt.Errorf(
				"invalid matrices[%q]: at least one of durations, distances or costs is required",
				profile,
			)
		}
		matrices[profile] = justrouting.CustomMatrix{
			Durations: m.Durations,
			Distances: m.Distances,
			Costs:     m.Costs,
		}
	}

	var options *justrouting.OptimizationOptions
	if input.Options != nil {
		options = &justrouting.OptimizationOptions{Geometry: input.Options.Geometry}
	}

	req := &justrouting.OptimizationRequest{
		Vehicles: vehicles,
		Matrices: matrices,
		Options:  options,
	}
	// Leave nil slices nil so the client omits the fields entirely.
	if len(jobs) > 0 {
		req.Jobs = jobs
	}
	if len(shipments) > 0 {
		req.Shipments = shipments
	}
	return req, nil
}

// checkAmountLengths ensures every amount vector in the request (vehicle
// capacity, job delivery/pickup, shipment amount, break max_load) shares one
// length, so the LLM gets a clearer message than the engine's "Inconsistent
// delivery length" error. The first non-empty vector is the reference.
func checkAmountLengths(input OptimizeInput) error {
	var reference []int
	refLabel := ""

	check := func(label string, values []int) error {
		if len(values) == 0 {
			return nil
		}
		if reference == nil {
			reference = values
			refLabel = label
			return nil
		}
		if len(values) != len(reference) {
			return fmt.Errorf(
				"invalid %s = %v: length %d does not match %s (length %d)",
				label, values, len(values), refLabel, len(reference),
			)
		}
		return nil
	}

	for i, v := range input.Vehicles {
		if err := check(fmt.Sprintf("vehicles[%d].capacity", i), v.Capacity); err != nil {
			return err
		}
		for k, b := range v.Breaks {
			if err := check(fmt.Sprintf("vehicles[%d].breaks[%d].max_load", i, k), b.MaxLoad); err != nil {
				return err
			}
		}
	}
	for i, j := range input.Jobs {
		if err := check(fmt.Sprintf("jobs[%d].delivery", i), j.Delivery); err != nil {
			return err
		}
		if err := check(fmt.Sprintf("jobs[%d].pickup", i), j.Pickup); err != nil {
			return err
		}
	}
	for i, s := range input.Shipments {
		if err := check(fmt.Sprintf("shipments[%d].amount", i), s.Amount); err != nil {
			return err
		}
	}
	return nil
}

// checkNonNegative rejects negative entries in an amount or skills vector,
// naming the offending index.
func checkNonNegative(label string, values []int) error {
	for i, v := range values {
		if v < 0 {
			return fmt.Errorf("invalid %s[%d] = %d: must not be negative", label, i, v)
		}
	}
	return nil
}

// checkTimeWindows rejects windows whose start is after their end.
func checkTimeWindows(label string, windows []OptimizeTimeWindow) error {
	for i, w := range windows {
		if w[0] > w[1] {
			return fmt.Errorf(
				"invalid %s[%d] = %v: start must be less than or equal to end",
				label, i, w,
			)
		}
	}
	return nil
}

// checkPriority rejects priorities outside the engine's 0-100 range.
func checkPriority(label string, priority int) error {
	if priority < 0 || priority > 100 {
		return fmt.Errorf("invalid %s.priority = %d: must be between 0 and 100", label, priority)
	}
	return nil
}

// optimizeTaskInput holds the fields jobs and shipment steps share, so one
// validation path serves both.
type optimizeTaskInput struct {
	Location       string
	LocationIndex  *int
	Setup          int
	Service        int
	SetupPerType   map[string]int
	ServicePerType map[string]int
	TimeWindows    []OptimizeTimeWindow
}

// buildOptimizeTaskLocation validates the fields shared by jobs and shipment
// steps, then parses the location. The location-or-index rule keeps custom
// matrix requests usable; the returned point is nil when only an index is
// given.
func buildOptimizeTaskLocation(label string, task optimizeTaskInput) (justrouting.Point, error) {
	if task.LocationIndex != nil && *task.LocationIndex < 0 {
		return nil, fmt.Errorf("invalid %s.location_index = %d: must not be negative", label, *task.LocationIndex)
	}
	if task.Setup < 0 {
		return nil, fmt.Errorf("invalid %s.setup = %d: must not be negative", label, task.Setup)
	}
	if task.Service < 0 {
		return nil, fmt.Errorf("invalid %s.service = %d: must not be negative", label, task.Service)
	}
	if err := checkNonNegative(label+".setup_per_type", perTypeValues(task.SetupPerType)); err != nil {
		return nil, err
	}
	if err := checkNonNegative(label+".service_per_type", perTypeValues(task.ServicePerType)); err != nil {
		return nil, err
	}
	if err := checkTimeWindows(label+".time_windows", task.TimeWindows); err != nil {
		return nil, err
	}

	if strings.TrimSpace(task.Location) != "" {
		point, err := parsePoint(task.Location)
		if err != nil {
			return nil, fmt.Errorf("invalid %s.location: %w", label, err)
		}
		return point, nil
	}
	if task.LocationIndex == nil {
		return nil, fmt.Errorf("invalid %s: a location or a location_index is required", label)
	}
	return nil, nil
}

// perTypeValues returns the values of a per-type map in a stable order, so
// validation messages do not depend on map iteration order.
func perTypeValues(perType map[string]int) []int {
	if len(perType) == 0 {
		return nil
	}
	values := make([]int, 0, len(perType))
	for _, v := range perType {
		values = append(values, v)
	}
	return values
}

// buildOptimizeBreaks validates and maps a vehicle's breaks. Break ids must
// be unique within their vehicle, matching the engine's rule.
func buildOptimizeBreaks(vehicle int, breaks []OptimizeBreak) ([]justrouting.Break, error) {
	if len(breaks) == 0 {
		return nil, nil
	}
	out := make([]justrouting.Break, len(breaks))
	seen := make(map[int]bool, len(breaks))
	for k, b := range breaks {
		label := fmt.Sprintf("vehicles[%d].breaks[%d]", vehicle, k)
		if b.ID <= 0 {
			return nil, fmt.Errorf("invalid %s.id = %d: must be a positive integer", label, b.ID)
		}
		if seen[b.ID] {
			return nil, fmt.Errorf("invalid %s.id = %d: break ids must be unique within the vehicle", label, b.ID)
		}
		seen[b.ID] = true
		if b.Service < 0 {
			return nil, fmt.Errorf("invalid %s.service = %d: must not be negative", label, b.Service)
		}
		if err := checkTimeWindows(label+".time_windows", b.TimeWindows); err != nil {
			return nil, err
		}
		if err := checkNonNegative(label+".max_load", b.MaxLoad); err != nil {
			return nil, err
		}

		out[k] = justrouting.Break{
			ID:          b.ID,
			TimeWindows: convertTimeWindows(b.TimeWindows),
			Service:     b.Service,
			Description: b.Description,
			MaxLoad:     b.MaxLoad,
		}
	}
	return out, nil
}

// buildOptimizeVehicleSteps validates and maps a vehicle's warm-start steps.
// The hosted API runs in solving mode, where only job, pickup and delivery
// steps are used.
func buildOptimizeVehicleSteps(vehicle int, steps []OptimizeVehicleStep) ([]justrouting.VehicleStep, error) {
	if len(steps) == 0 {
		return nil, nil
	}
	out := make([]justrouting.VehicleStep, len(steps))
	seen := make(map[int]bool, len(steps))
	for k, s := range steps {
		label := fmt.Sprintf("vehicles[%d].steps[%d]", vehicle, k)
		switch s.Type {
		case "job", "pickup", "delivery":
		default:
			return nil, fmt.Errorf(
				"invalid %s.type = %q: only \"job\", \"pickup\" and \"delivery\" steps are used in solving mode",
				label, s.Type,
			)
		}
		if s.ID <= 0 {
			return nil, fmt.Errorf("invalid %s.id = %d: must be a positive integer", label, s.ID)
		}
		if seen[s.ID] {
			return nil, fmt.Errorf("invalid %s.id = %d: step ids must be unique within the vehicle", label, s.ID)
		}
		seen[s.ID] = true
		out[k] = justrouting.VehicleStep{Type: s.Type, ID: s.ID}
	}
	return out, nil
}

// buildOptimizeShipmentStep validates and maps one half of a shipment.
// Step ids are unique within their kind — pickups among pickups, deliveries
// among deliveries — matching the engine's rule.
func buildOptimizeShipmentStep(
	label string,
	kind string,
	step OptimizeShipmentStep,
	seen map[int]bool,
) (justrouting.ShipmentStep, error) {
	if step.ID <= 0 {
		return justrouting.ShipmentStep{}, fmt.Errorf("invalid %s.id = %d: must be a positive integer", label, step.ID)
	}
	if seen[step.ID] {
		return justrouting.ShipmentStep{}, fmt.Errorf("invalid %s.id = %d: %s ids must be unique", label, step.ID, kind)
	}
	seen[step.ID] = true

	location, err := buildOptimizeTaskLocation(label, optimizeTaskInput{
		Location:       step.Location,
		LocationIndex:  step.LocationIndex,
		Setup:          step.Setup,
		Service:        step.Service,
		SetupPerType:   step.SetupPerType,
		ServicePerType: step.ServicePerType,
		TimeWindows:    step.TimeWindows,
	})
	if err != nil {
		return justrouting.ShipmentStep{}, err
	}

	return justrouting.ShipmentStep{
		ID:             step.ID,
		Description:    step.Description,
		Location:       location,
		LocationIndex:  step.LocationIndex,
		Setup:          step.Setup,
		Service:        step.Service,
		SetupPerType:   step.SetupPerType,
		ServicePerType: step.ServicePerType,
		TimeWindows:    convertTimeWindows(step.TimeWindows),
	}, nil
}

// convertTimeWindows maps the tool's window type onto the client's.
func convertTimeWindows(in []OptimizeTimeWindow) []justrouting.TimeWindow {
	if len(in) == 0 {
		return nil
	}
	out := make([]justrouting.TimeWindow, len(in))
	for i, w := range in {
		out[i] = justrouting.TimeWindow(w)
	}
	return out
}

// optimizeProfile maps a normalized Profile onto the name the optimization
// engine expects. The engine's default is "car" rather than "driving", so a
// driving profile must be sent as "car" to preserve the engine default.
func optimizeProfile(profile Profile) string {
	if profile == MotorcycleProfile {
		return "motorcycle"
	}
	return "car"
}

// buildOptimizeOutput maps the client solution onto the tool's own output
// structs. The mapping is nil-safe: the engine may return nil routes, steps
// or unassigned entries.
func buildOptimizeOutput(s *justrouting.Solution) OptimizeOutput {
	out := OptimizeOutput{
		Summary: OptimizeSummary{
			Cost:        s.Summary.Cost,
			Routes:      s.Summary.Routes,
			Unassigned:  s.Summary.Unassigned,
			Setup:       s.Summary.Setup,
			Service:     s.Summary.Service,
			Duration:    s.Summary.Duration,
			WaitingTime: s.Summary.WaitingTime,
			Priority:    s.Summary.Priority,
			Distance:    s.Summary.Distance,
			Delivery:    s.Summary.Delivery,
			Pickup:      s.Summary.Pickup,
			Violations:  buildOptimizeViolations(s.Summary.Violations),
		},
	}

	out.Routes = make([]OptimizeRoute, 0, len(s.Routes))
	for _, r := range s.Routes {
		if r == nil {
			continue
		}
		route := OptimizeRoute{
			Vehicle:     r.Vehicle,
			Cost:        r.Cost,
			Setup:       r.Setup,
			Service:     r.Service,
			Duration:    r.Duration,
			WaitingTime: r.WaitingTime,
			Priority:    r.Priority,
			Distance:    r.Distance,
			Delivery:    r.Delivery,
			Pickup:      r.Pickup,
			Description: r.Description,
			Geometry:    r.Geometry,
			Violations:  buildOptimizeViolations(r.Violations),
		}
		route.Steps = make([]OptimizeStep, 0, len(r.Steps))
		for _, st := range r.Steps {
			if st == nil {
				continue
			}
			route.Steps = append(route.Steps, OptimizeStep{
				Type:          st.Type,
				Location:      st.Location,
				LocationIndex: st.LocationIndex,
				Job:           st.Job,
				ID:            st.ID,
				Setup:         st.Setup,
				Service:       st.Service,
				WaitingTime:   st.WaitingTime,
				Arrival:       st.Arrival,
				Duration:      st.Duration,
				Distance:      st.Distance,
				Load:          st.Load,
				Description:   st.Description,
				Violations:    buildOptimizeViolations(st.Violations),
			})
		}
		out.Routes = append(out.Routes, route)
	}

	out.Unassigned = make([]OptimizeUnassigned, 0, len(s.Unassigned))
	for _, u := range s.Unassigned {
		if u == nil {
			continue
		}
		out.Unassigned = append(out.Unassigned, OptimizeUnassigned{
			ID:            u.ID,
			Type:          u.Type,
			Location:      u.Location,
			LocationIndex: u.LocationIndex,
			Description:   u.Description,
		})
	}

	return out
}

func buildOptimizeViolations(in []justrouting.Violation) []OptimizeViolation {
	if len(in) == 0 {
		return nil
	}
	out := make([]OptimizeViolation, len(in))
	for i, v := range in {
		out[i] = OptimizeViolation{Cause: v.Cause, Duration: v.Duration}
	}
	return out
}
