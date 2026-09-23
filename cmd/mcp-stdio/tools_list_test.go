package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

const serverGolden = "../../internal/server/testdata/tools_list.golden.json"

// The released stdio binary and the HTTP server must publish one list: one golden file answers for both.
func TestStdioToolsListMatchesTheServerGolden(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "mcp-stdio")
	// #nosec G204 -- the test builds this package into its own temp directory and runs that
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	cmd := exec.Command(bin) // #nosec G204 -- bin is the binary this test just built
	cmd.Env = append(os.Environ(),
		"FRAPPE_BASE_URL=http://frappe.invalid", "FRAPPE_API_KEY=k", "FRAPPE_API_SECRET=s",
		"TOOLS_KNOWLEDGE_BASE=true")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	lines := bufio.NewScanner(stdout)
	lines.Buffer(make([]byte, 0, 64*1024), 1<<20)
	send := func(request string) {
		if _, err := stdin.Write([]byte(request + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	read := func() []byte {
		if !lines.Scan() {
			t.Fatalf("stdio server closed: %v", lines.Err())
		}
		return lines.Bytes()
	}

	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18",` +
		`"capabilities":{},"clientInfo":{"name":"golden","version":"0"}}}`)
	read()
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)

	var response struct {
		Result struct {
			Tools []map[string]any `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(read(), &response); err != nil {
		t.Fatal(err)
	}
	tools := response.Result.Tools
	sort.Slice(tools, func(i, j int) bool { return tools[i]["name"].(string) < tools[j]["name"].(string) })
	got, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile(serverGolden)
	if err != nil {
		t.Fatal(err)
	}
	var wantAny, gotAny any
	if err := json.Unmarshal(want, &wantAny); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got, &gotAny); err != nil {
		t.Fatal(err)
	}
	wantJSON, _ := json.Marshal(wantAny)
	gotJSON, _ := json.Marshal(gotAny)
	if string(wantJSON) != string(gotJSON) {
		t.Errorf("stdio tools/list differs from %s\n got: %s\nwant: %s", serverGolden, got, want)
	}
}
