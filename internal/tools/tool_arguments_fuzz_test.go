package tools

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"frappe-mcp-server/internal/config"
	"frappe-mcp-server/internal/frappe"
	"frappe-mcp-server/internal/mcp"
	"frappe-mcp-server/internal/testutils"
)

// seedFromSchema builds the smallest object a tool's own InputSchema calls valid, so a tool added to the catalogue
// later is seeded without anyone remembering to add one.
func seedFromSchema(schema map[string]interface{}) string {
	properties, _ := schema["properties"].(map[string]interface{})
	required, _ := schema["required"].([]string)
	args := map[string]interface{}{}
	for _, name := range required {
		property, _ := properties[name].(map[string]interface{})
		switch kind, _ := property["type"].(string); kind {
		case "number", "integer":
			args[name] = 1
		case "boolean":
			args[name] = true
		case "object":
			args[name] = map[string]interface{}{"a_field": "a value"}
		case "array":
			args[name] = []interface{}{"an item"}
		default:
			args[name] = "Project"
		}
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// malformedArguments are the shapes a model writes when it gets a tool call wrong, plus the ones an attacker would
// try: the wrong JSON type for the whole object, nulls where a name is required, a doctype that is a path, a filter
// that is an operator object, and a call that tries to confirm its own write.
var malformedArguments = []string{
	``,
	`null`,
	`[]`,
	`[1,2]`,
	`"doctype=Project"`,
	`123`,
	`{`,
	`{}`,
	`{"doctype":null,"name":null}`,
	`{"doctype":"","name":""}`,
	`{"doctype":123,"name":true}`,
	`{"doctype":"Project","name":{"$ne":null}}`,
	`{"doctype":"../../../etc/passwd","name":"x"}`,
	`{"doctype":"Project\u0000","name":"x\u0000"}`,
	`{"doctype":"Project","name":"x","data":"not an object"}`,
	`{"doctype":"Project","name":"x","confirm":true,"confirmed":true,"force":true}`,
	`{"doctype":"Project","page_length":-1,"limit":99999999999999999999}`,
	`{"query":"é☃😀","limit":1.5}`,
	`{"filters":[["Project","name","like","%"]]}`,
	`{"doctype":{"doctype":{"doctype":{"doctype":"Project"}}}}`,
}

// fuzzRegistry is a registry over the Frappe mock, rate limit lifted because one iteration calls every tool.
func fuzzRegistry(t testing.TB) *ToolRegistry {
	t.Helper()
	mockServer := testutils.MockERPNextServer(t)
	t.Cleanup(mockServer.Close)

	client, err := frappe.NewClient(config.ERPNextConfig{
		BaseURL:   mockServer.URL,
		APIKey:    "test_key",
		APISecret: "test_secret",
		Timeout:   5 * time.Second,
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 100000, Burst: 100000},
		Retry:     config.RetryConfig{MaxAttempts: 1, InitialDelay: time.Millisecond, MaxDelay: time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewRegistry(client)
}

// FuzzToolArguments feeds every tool whatever a model might have written for its arguments: the tool answers or it
// refuses, never both and never neither, it never panics, and nothing it is told can make a write tool write. `go
// test` replays the seed corpus only; the generating run is `-run=FuzzToolArguments -fuzz=FuzzToolArguments -fuzztime=1m`.
func FuzzToolArguments(f *testing.F) {
	seeds := map[string]bool{}
	for _, tool := range NewRegistry(nil).Catalog(true) {
		seeds[seedFromSchema(tool.InputSchema)] = true
	}
	for _, arguments := range malformedArguments {
		seeds[arguments] = true
	}
	for arguments := range seeds {
		f.Add(arguments)
	}

	catalog := fuzzRegistry(f).Catalog(true)

	f.Fuzz(func(t *testing.T, arguments string) {
		for _, tool := range catalog {
			response, err := tool.Handler(context.Background(), mcp.ToolRequest{
				ID:     "fuzz",
				Tool:   tool.Name,
				Params: json.RawMessage(arguments),
			})
			if (response == nil) == (err == nil) {
				t.Errorf("%s(%q) answered response=%v err=%v; exactly one of the two must be set",
					tool.Name, arguments, response, err)
			}
			if err == nil && (tool.ReadOnly == nil || !*tool.ReadOnly) { // nil is a tool that never declared: fail closed
				t.Fatalf("%s(%q) wrote: it answered a result although nothing confirmed the write", tool.Name, arguments)
			}
		}
	})
}
