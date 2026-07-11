package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpTestHelperDB = "VECLITE_MCP_TEST_HELPER_DB"

// TestMain turns the current test binary into the real VecLite stdio server
// for TestMCPStdioProtocol. Keeping the helper in-process with the package
// exercises cmdMCP and its production tool registration while still crossing
// an actual subprocess/stdin/stdout transport boundary.
func TestMain(m *testing.M) {
	if dbPath := os.Getenv(mcpTestHelperDB); dbPath != "" {
		if err := cmdMCP([]string{dbPath}); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "veclite MCP test helper: %v\n", err)
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestMCPStdioProtocol(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	dbPath := t.TempDir() + "/protocol.veclite"
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), mcpTestHelperDB+"="+dbPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "veclite-protocol-test",
		Version: "1.0.0",
	}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{
		Command:           cmd,
		TerminateDuration: 2 * time.Second,
	}, nil)
	if err != nil {
		t.Fatalf("initialize MCP stdio session: %v (stderr: %s)", err, stderr.String())
	}
	closed := false
	defer func() {
		if !closed {
			_ = session.Close()
		}
	}()

	initResult := session.InitializeResult()
	if initResult == nil {
		t.Fatal("initialize returned no result")
	}
	if got, want := initResult.ProtocolVersion, "2025-11-25"; got != want {
		t.Fatalf("negotiated protocol version = %q, want %q", got, want)
	}
	if initResult.ServerInfo == nil || initResult.ServerInfo.Name != "veclite" {
		t.Fatalf("server info = %#v, want veclite implementation", initResult.ServerInfo)
	}

	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list MCP tools: %v", err)
	}
	for _, want := range []string{"veclite_collections", "veclite_collection_schema", "veclite_create_collection", "memory_find_clusters"} {
		var found *mcp.Tool
		for _, tool := range listed.Tools {
			if tool.Name == want {
				found = tool
				break
			}
		}
		if found == nil {
			t.Fatalf("tools/list omitted %q", want)
		}
		if want == "veclite_collection_schema" || want == "memory_find_clusters" {
			schemaJSON, err := json.Marshal(found.InputSchema)
			if err != nil {
				t.Fatalf("encode %s input schema: %v", want, err)
			}
			var schema struct {
				Required []string `json:"required"`
			}
			if err := json.Unmarshal(schemaJSON, &schema); err != nil {
				t.Fatalf("decode %s input schema: %v", want, err)
			}
			required := false
			for _, name := range schema.Required {
				if name == "collection" {
					required = true
					break
				}
			}
			if want == "veclite_collection_schema" && !required {
				t.Fatal("veclite_collection_schema input schema does not require collection")
			}
			if want == "memory_find_clusters" && required {
				t.Fatal("memory_find_clusters input schema requires collection despite its memories default")
			}
		}
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "veclite_collections",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("call veclite_collections: %v", err)
	}
	if result.IsError {
		t.Fatalf("veclite_collections returned tool error: %#v", result.Content)
	}
	if len(result.Content) != 1 {
		t.Fatalf("veclite_collections content items = %d, want 1", len(result.Content))
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok || strings.TrimSpace(text.Text) != "[]" {
		t.Fatalf("veclite_collections content = %#v, want empty JSON array", result.Content[0])
	}

	created, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "veclite_create_collection",
		Arguments: map[string]any{
			"name":      "documents",
			"dimension": 3,
		},
	})
	if err != nil {
		t.Fatalf("call veclite_create_collection: %v", err)
	}
	if created.IsError {
		t.Fatalf("veclite_create_collection returned tool error: %#v", created.Content)
	}

	result, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "veclite_collections",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("list collections after create: %v", err)
	}
	if result.IsError || len(result.Content) != 1 {
		t.Fatalf("list collections after create returned %#v", result)
	}
	text, ok = result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("list collections content type = %T, want TextContent", result.Content[0])
	}
	var collections []struct {
		Name      string `json:"name"`
		Dimension int    `json:"dimension"`
	}
	if err := json.Unmarshal([]byte(text.Text), &collections); err != nil {
		t.Fatalf("decode collections result: %v", err)
	}
	if len(collections) != 1 || collections[0].Name != "documents" || collections[0].Dimension != 3 {
		t.Fatalf("collections after typed create = %#v, want documents dimension 3", collections)
	}

	invalid, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "veclite_collection_schema",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("invalid input returned JSON-RPC error: %v; want IsError tool result", err)
	}
	if !invalid.IsError {
		t.Fatal("invalid required input returned IsError=false")
	}
	if len(invalid.Content) == 0 {
		t.Fatal("invalid required input returned no explanatory content")
	}
	invalidText, ok := invalid.Content[0].(*mcp.TextContent)
	if !ok || !strings.Contains(strings.ToLower(invalidText.Text), "collection") {
		t.Fatalf("invalid-input content = %#v, want missing collection explanation", invalid.Content[0])
	}

	if err := session.Close(); err != nil {
		t.Fatalf("close MCP stdio session: %v (stderr: %s)", err, stderr.String())
	}
	closed = true
	if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
		t.Fatalf("MCP server did not exit cleanly: process state %#v", cmd.ProcessState)
	}
}
