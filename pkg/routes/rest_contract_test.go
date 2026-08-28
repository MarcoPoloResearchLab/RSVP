package routes_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tyemirov/RSVP/pkg/config"
)

func TestHorizonOpenAPIContainsCanonicalResourceSurface(testingContext *testing.T) {
	_, sourcePath, _, sourceFound := runtime.Caller(0)
	if !sourceFound {
		testingContext.Fatal("locate route contract test")
	}
	contractPath := filepath.Join(filepath.Dir(sourcePath), "..", "..", "api", "horizon.openapi.json")
	contractBytes, readError := os.ReadFile(contractPath)
	if readError != nil {
		testingContext.Fatalf("read Horizon OpenAPI contract: %v", readError)
	}
	var contract struct {
		OpenAPI string                                `json:"openapi"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if decodeError := json.Unmarshal(contractBytes, &contract); decodeError != nil {
		testingContext.Fatalf("decode Horizon OpenAPI contract: %v", decodeError)
	}
	if contract.OpenAPI != "3.1.0" {
		testingContext.Fatalf("OpenAPI version = %q", contract.OpenAPI)
	}
	required := map[string][]string{
		config.WebHorizon:                                                   {"get", "head"},
		config.WebCalendars:                                                 {"post"},
		config.WebCalendars + "{calendar_id}":                               {"get", "patch", "delete"},
		config.WebLanes:                                                     {"post"},
		config.WebLanes + "{lane_id}":                                       {"get", "patch", "delete"},
		config.WebAttentionPolicies:                                         {"post"},
		config.WebAttentionPolicies + "{policy_id}":                         {"patch", "delete"},
		config.WebProbes + "{probe_id}":                                     {"patch"},
		config.WebCalendarAuthorizationRequests:                             {"post"},
		config.WebCalendarConnectionCallbacksGoogle:                         {"get"},
		config.WebCalendarConnections:                                       {"post"},
		config.WebCalendarConnections + "{connection_id}":                   {"get", "delete"},
		config.WebCalendarConnections + "{connection_id}/source-calendars/": {"get"},
		config.WebDerivedMarkerRules:                                        {"post"},
		config.WebDerivedMarkerRules + "{rule_id}":                          {"get", "patch", "delete"},
		config.WebIngestionDrafts:                                           {"post"},
		config.WebIngestionDrafts + "{draft_id}":                            {"get", "patch", "delete"},
		config.WebIngestionDrafts + "{draft_id}/confirmations/":             {"post"},
	}
	if len(contract.Paths) != len(required) {
		testingContext.Fatalf("OpenAPI path count = %d, want %d", len(contract.Paths), len(required))
	}
	for path, methods := range required {
		operations, pathFound := contract.Paths[path]
		if !pathFound {
			testingContext.Errorf("OpenAPI path %q is absent", path)
			continue
		}
		if len(operations) != len(methods) {
			testingContext.Errorf("OpenAPI methods for %q = %#v, want %#v", path, operations, methods)
			continue
		}
		for _, method := range methods {
			if operations[method] == nil {
				testingContext.Errorf("OpenAPI operation %s %s is absent", method, path)
			}
		}
	}
}
