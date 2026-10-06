package tools

import (
	"context"
	"fmt"
	"strings"

	justrouting "github.com/justrouting/go-client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// maxGeocodeResults caps how many geocode results the tool returns, keeping
// responses small enough for LLM context.
const maxGeocodeResults = 10

type GeocodeConfig struct {
	APIKey string
}

type GeocodeInput struct {
	Name        string   `json:"name,omitempty" jsonschema:"name of the place, for example \"Marina Bay Sands\" or \"marina bay\""`
	Country     string   `json:"country,omitempty" jsonschema:"country, for example \"Singapore\""`
	City        string   `json:"city,omitempty" jsonschema:"city or locality, for example \"Singapore\""`
	Street      string   `json:"street,omitempty" jsonschema:"street name, for example \"Bayfront Avenue\""`
	Housenumber string   `json:"housenumber,omitempty" jsonschema:"house or building number, for example \"10\""`
	Postcode    string   `json:"postcode,omitempty" jsonschema:"postal code, for example \"018956\""`
	Limit       int      `json:"limit,omitempty" jsonschema:"maximum number of results to return; defaults to 1 (best match only), must not exceed 10"`
	Filters     []string `json:"filters,omitempty" jsonschema:"optional filters to restrict results, for example \"countrycode:sg\""`
}

type GeocodeResult struct {
	Longitude   float64 `json:"longitude"`
	Latitude    float64 `json:"latitude"`
	Coordinates string  `json:"coordinates"`
	Formatted   string  `json:"formatted,omitempty"`
	PlaceID     string  `json:"place_id,omitempty"`
	CountryCode string  `json:"country_code,omitempty"`
	ResultType  string  `json:"result_type,omitempty"`
}

type GeocodeOutput struct {
	Results []GeocodeResult `json:"results"`
}

func RegisterGeocodeTool(
	server *mcp.Server,
	cfg GeocodeConfig,
) {
	client := justrouting.NewClient(cfg.APIKey)

	mcp.AddTool(
		server,
		&mcp.Tool{
			Name: "geocode",
			Description: `
Search for a place or address and convert it into coordinates using JustRouting.

Use this tool when the user provides a place name, landmark, business,
street address, or other location description instead of coordinates.

Do not use this tool when the user already provides coordinates in
longitude,latitude format.

Parse the user's location into the structured fields that can be determined:
- "name": place, landmark, or business name
- "housenumber": house or building number
- "street": street name
- "postcode": postal or ZIP code
- "city": city or locality
- "country": country

Only provide fields that are known from the user's request. Do not invent or
guess missing address components.

For example, "Marina Bay Sands, 10 Bayfront Avenue, Singapore 018956"
can be represented as:
- name: "Marina Bay Sands"
- housenumber: "10"
- street: "Bayfront Avenue"
- postcode: "018956"
- city: "Singapore"
- country: "Singapore"

When the user's context provides a country or city, include it to disambiguate
common, abbreviated, or misspelled place names.

Results are ordered by relevance. Each result contains longitude, latitude,
and a "coordinates" string in longitude,latitude format that can be passed
directly to the route, table, or optimize tools.

By default, only the best matching result is returned. If the location is
ambiguous or the user asks for alternatives, request multiple results using
"limit" and use the result that best matches the user's intended location.

When using geocode as part of a route, table, or optimize workflow, verify
that the selected result matches the user's intended place before passing
its coordinates to the next tool.
			`,
		},
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			input GeocodeInput,
		) (*mcp.CallToolResult, GeocodeOutput, error) {
			return searchGeocode(ctx, client, input)
		},
	)
}

func searchGeocode(
	ctx context.Context,
	client *justrouting.Client,
	input GeocodeInput,
) (*mcp.CallToolResult, GeocodeOutput, error) {
	// Components the LLM could not determine stay empty, and the go-client
	// drops empty fields from the query, so the API only sees what was
	// actually provided.
	structured := &justrouting.StructuredQuery{
		Name:        strings.TrimSpace(input.Name),
		Housenumber: strings.TrimSpace(input.Housenumber),
		Street:      strings.TrimSpace(input.Street),
		Postcode:    strings.TrimSpace(input.Postcode),
		City:        strings.TrimSpace(input.City),
		Country:     strings.TrimSpace(input.Country),
	}
	if describeStructured(structured) == "" {
		return nil, GeocodeOutput{}, fmt.Errorf(
			"at least one of name, housenumber, street, postcode, city, country is required",
		)
	}

	// Validate the limit here so the LLM gets a clear message instead of
	// the client's validation error, and so responses stay bounded.
	if input.Limit < 0 {
		return nil, GeocodeOutput{}, fmt.Errorf(
			"limit must not be negative, got %d",
			input.Limit,
		)
	}
	if input.Limit > maxGeocodeResults {
		return nil, GeocodeOutput{}, fmt.Errorf(
			"limit must not exceed %d, got %d",
			maxGeocodeResults,
			input.Limit,
		)
	}

	// An omitted limit defaults to 1 so the tool returns only the best
	// match instead of the API's default of 5.
	limit := input.Limit
	if limit == 0 {
		limit = 1
	}

	resp, err := client.Geocode.Search(
		ctx,
		&justrouting.GeocodeRequest{
			Structured: structured,
			Limit:      limit,
			Filters:    input.Filters,
		},
	)
	if err != nil {
		return nil, GeocodeOutput{}, fmt.Errorf("geocode search failed: %w", err)
	}

	// An empty result list is a valid API response; surface it to the LLM
	// as an error so it retries with a different query instead of passing
	// nothing to the route tool.
	if len(resp.Results) == 0 {
		return nil, GeocodeOutput{}, fmt.Errorf(
			"no results found for %s",
			describeStructured(structured),
		)
	}

	results := make([]GeocodeResult, 0, len(resp.Results))
	for _, r := range resp.Results {
		results = append(results, GeocodeResult{
			Longitude:   r.Lon,
			Latitude:    r.Lat,
			Coordinates: r.Location().String(),
			Formatted:   r.Formatted,
			PlaceID:     r.PlaceID,
			CountryCode: r.CountryCode,
			ResultType:  r.ResultType,
		})
	}

	return nil, GeocodeOutput{Results: results}, nil
}

// describeStructured renders the non-empty components of a structured query
// as key=value pairs, used in error messages so the LLM sees which
// components were searched.
func describeStructured(s *justrouting.StructuredQuery) string {
	parts := make([]string, 0, 6)
	add := func(key, v string) {
		if v != "" {
			parts = append(parts, fmt.Sprintf("%s=%q", key, v))
		}
	}
	add("name", s.Name)
	add("housenumber", s.Housenumber)
	add("street", s.Street)
	add("postcode", s.Postcode)
	add("city", s.City)
	add("country", s.Country)
	return strings.Join(parts, ", ")
}
