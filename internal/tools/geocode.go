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
	Text    string   `json:"text" jsonschema:"address or place name to search, for example \"marina bay singapore\""`
	Limit   int      `json:"limit,omitempty" jsonschema:"maximum number of results to return; defaults to 1 (best match only), must not exceed 10"`
	Filters []string `json:"filters,omitempty" jsonschema:"optional filters to restrict results, for example \"countrycode:sg\""`
}

type GeocodeResultOutput struct {
	Longitude   float64 `json:"longitude"`
	Latitude    float64 `json:"latitude"`
	Coordinates string  `json:"coordinates"`
	Formatted   string  `json:"formatted,omitempty"`
	PlaceID     string  `json:"place_id,omitempty"`
	CountryCode string  `json:"country_code,omitempty"`
	ResultType  string  `json:"result_type,omitempty"`
}

type GeocodeOutput struct {
	Results []GeocodeResultOutput `json:"results"`
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
Search for places and convert an address or place name into coordinates using JustRouting.

Returns a list of matching places, ordered by relevance. Each result includes
longitude, latitude, and a ready-to-use "coordinates" string in longitude,latitude
format that can be passed directly to the route tool.

Use this tool first when the user refers to places by name or address (for example
"marina bay singapore"). By default only the best match is returned; if the result
looks ambiguous, raise "limit" and use the formatted address and country code to
pick the most plausible result, or ask the user to clarify.
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
	text := strings.TrimSpace(input.Text)
	if text == "" {
		return nil, GeocodeOutput{}, fmt.Errorf("text is required")
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
			Text:    text,
			Limit:   limit,
			Filters: input.Filters,
		},
	)
	if err != nil {
		return nil, GeocodeOutput{}, fmt.Errorf("geocode search failed: %w", err)
	}

	// An empty result list is a valid API response; surface it to the LLM
	// as an error so it retries with a different query instead of passing
	// nothing to the route tool.
	if len(resp.Results) == 0 {
		return nil, GeocodeOutput{}, fmt.Errorf("no results found for %q", text)
	}

	results := make([]GeocodeResultOutput, 0, len(resp.Results))
	for _, r := range resp.Results {
		results = append(results, GeocodeResultOutput{
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
