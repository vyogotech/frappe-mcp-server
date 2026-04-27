package server

// sse.go — Server-Sent Events support for /api/v1/chat.
//
// When a request arrives with `Accept: text/event-stream`, handleChat
// dispatches to handleChatSSE instead of the JSON handler.
//
// SSE event schema (all payloads are JSON, format: "data: <json>\n\n"):
//
//	{"type":"status",    "message":"..."}
//	{"type":"tool_call", "name":"...", "arguments":{...}}
//	{"type":"content",   "text":"token"}   // one per LLM token
//	{"type":"done",      "tools_called":[], "data_quality":"high", "timestamp":"..."}
//	{"type":"error",     "message":"..."}

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"frappe-mcp-server/internal/llm"
	"frappe-mcp-server/internal/mcp"
)

// sseEvent is the envelope written for every SSE message.
type sseEvent struct {
	Type      string          `json:"type"`
	Message   string          `json:"message,omitempty"`
	Name      string          `json:"name,omitempty"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	Text      string          `json:"text,omitempty"`
	// done fields
	ToolsCalled []string `json:"tools_called,omitempty"`
	DataQuality string   `json:"data_quality,omitempty"`
	Timestamp   string   `json:"timestamp,omitempty"`
}

// sseWriter wraps a ResponseWriter/Flusher so callers can emit events with a
// single call.  It is NOT safe for concurrent writes (the handler is linear).
type sseWriter struct {
	w http.ResponseWriter
	f http.Flusher
}

func newSSEWriter(w http.ResponseWriter) (*sseWriter, bool) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	return &sseWriter{w: w, f: f}, true
}

func (sw *sseWriter) emit(ev sseEvent) {
	b, err := json.Marshal(ev)
	if err != nil {
		slog.Warn("sse: failed to marshal event", "type", ev.Type, "error", err)
		return
	}
	if _, err := fmt.Fprintf(sw.w, "data: %s\n\n", b); err != nil {
		slog.Warn("sse: failed to write event (client likely disconnected)", "type", ev.Type, "error", err)
		return
	}
	sw.f.Flush()
}

func (sw *sseWriter) status(msg string) {
	sw.emit(sseEvent{Type: "status", Message: msg})
}

func (sw *sseWriter) toolCall(name string, args json.RawMessage) {
	sw.emit(sseEvent{Type: "tool_call", Name: name, Arguments: args})
}

func (sw *sseWriter) content(token string) {
	sw.emit(sseEvent{Type: "content", Text: token})
}

func (sw *sseWriter) done(toolsCalled []string, quality string) {
	sw.emit(sseEvent{
		Type:        "done",
		ToolsCalled: toolsCalled,
		DataQuality: quality,
		Timestamp:   time.Now().Format(time.RFC3339),
	})
}

func (sw *sseWriter) errEvent(msg string) {
	sw.emit(sseEvent{Type: "error", Message: msg})
}

// handleChatSSE is the streaming version of handleChat.
// It sets SSE headers, disables the write-deadline, then walks the same
// intent-extract → tool-execute → LLM-format pipeline while emitting events
// at each step so the browser can start rendering immediately.
func (s *MCPServer) handleChatSSE(w http.ResponseWriter, r *http.Request) {
	sw, ok := newSSEWriter(w)
	if !ok {
		http.Error(w, "SSE not supported by this server", http.StatusInternalServerError)
		return
	}

	// SSE headers.
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // tell nginx/proxy not to buffer
	w.WriteHeader(http.StatusOK)

	// Remove the server write deadline so long LLM responses don't get cut off.
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	// ── Parse request ────────────────────────────────────────────────────────
	var req struct {
		Message string `json:"message"`
		Model   string `json:"model,omitempty"`
		Context struct {
			UserID    string `json:"user_id"`
			UserEmail string `json:"user_email"`
			Timestamp string `json:"timestamp"`
		} `json:"context,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sw.errEvent("Invalid request body: " + err.Error())
		return
	}
	if req.Message == "" {
		sw.errEvent("Message is required")
		return
	}

	ctx := r.Context()
	sw.status("Analyzing your query…")

	// ── Intent extraction ────────────────────────────────────────────────────
	queryIntent, err := s.extractQueryIntent(ctx, req.Message)
	if err != nil {
		if strings.Contains(err.Error(), "rate limit") || strings.Contains(err.Error(), "429") {
			sw.errEvent("⚠️ The AI service is temporarily unavailable due to rate limits. Please try again in a few minutes.")
			sw.done(nil, "rate_limited")
			return
		}
		slog.Warn("Intent extraction failed, falling back to simple routing", "error", err)
		queryIntent = s.fallbackQueryRouting(req.Message)
	}

	slog.Info("SSE intent extracted",
		"action", queryIntent.Action,
		"doctype", queryIntent.DocType,
		"is_erpnext_related", queryIntent.IsERPNextRelated)

	if !queryIntent.IsERPNextRelated {
		sw.content("I'm an ERPNext assistant specialized in helping you with your business data (customers, invoices, projects, items, etc.). For general questions or other topics, please use a general-purpose AI assistant.")
		sw.done(nil, "not_applicable")
		return
	}

	// ── Tool execution ───────────────────────────────────────────────────────
	sw.status("Fetching data from ERPNext…")

	var (
		result      *mcp.ToolResponse
		toolsCalled []string
		toolErr     error
	)

	result, toolsCalled, toolErr = s.executeIntentSSE(ctx, sw, req.Message, queryIntent)

	// ── Response formatting ──────────────────────────────────────────────────
	if toolErr != nil {
		errMsg := toolErr.Error()

		// Missing report parameters — ask the user conversationally.
		if strings.HasPrefix(errMsg, "missing_params:") {
			parts := strings.SplitN(errMsg, ":", 3)
			if len(parts) == 3 {
				reportName := parts[1]
				missingParams := strings.Trim(parts[2], "[]")
				friendlyMsg := fmt.Sprintf(
					"To generate the **%s** report I need a bit more information: %s.\n\nPlease include these details in your request.",
					reportName, missingParams,
				)
				if s.llmManager != nil {
					prompt := fmt.Sprintf(`The user asked: "%s"

They want the "%s" report, but it requires: %s

Write a brief, friendly message asking for the missing parameters with 2-3 example queries.`,
						req.Message, reportName, missingParams)
					if text, err := s.generateWithLLM(ctx, prompt); err == nil {
						friendlyMsg = text
					}
				}
				sw.content(friendlyMsg)
				sw.done(toolsCalled, "needs_info")
				return
			}
		}

		formatted := formatFrappeError(errMsg, req.Message)
		sw.content(formatted)
		sw.done(toolsCalled, "error")
		return
	}

	if result == nil {
		sw.content("I understand you're asking about ERPNext data. Please be more specific about what information you need.")
		sw.done(toolsCalled, "low")
		return
	}

	// Extract raw text from tool response.
	var rawBuilder strings.Builder
	for _, c := range result.Content {
		if c.Type == "text" {
			rawBuilder.WriteString(c.Text)
			rawBuilder.WriteString("\n")
		}
	}
	rawText := strings.TrimSpace(rawBuilder.String())

	if rawText == "" {
		sw.content(formatEmptyResult(req.Message))
		sw.done(toolsCalled, "low")
		return
	}

	sw.status("Generating response…")

	// Try streaming from LLM.
	if streamed := s.streamFormatResponse(ctx, sw, req.Message, rawText); streamed {
		quality := dataQuality(rawText)
		sw.done(toolsCalled, quality)
		return
	}

	// Non-streaming fallback.
	var formattedResponse string
	if s.llmManager != nil || s.llmClient != nil {
		if text, err := s.formatResponseWithLLM(ctx, req.Message, rawText); err == nil {
			formattedResponse = text
		} else {
			formattedResponse = formatDataWithoutLLM(req.Message, rawText)
		}
	} else {
		formattedResponse = formatDataWithoutLLM(req.Message, rawText)
	}

	sw.content(formattedResponse)
	sw.done(toolsCalled, dataQuality(formattedResponse))
}

// executeIntentSSE mirrors the tool-dispatch logic in handleChatJSON but emits
// tool_call SSE events before executing each tool.
func (s *MCPServer) executeIntentSSE(
	ctx context.Context,
	sw *sseWriter,
	message string,
	intent *QueryIntent,
) (*mcp.ToolResponse, []string, error) {
	var toolsCalled []string

	emitTool := func(name string, params json.RawMessage) {
		sw.toolCall(name, params)
		toolsCalled = append(toolsCalled, name)
	}

	switch intent.Action {
	case "create":
		params, err := s.extractCreateParams(ctx, message, intent.DocType)
		if err != nil {
			return nil, toolsCalled, fmt.Errorf("failed to process create query: %w", err)
		}
		emitTool("create_document", params)
		result, err := s.executeTool(ctx, "create_document", params)
		return result, toolsCalled, err

	case "update":
		params, err := s.extractUpdateParams(ctx, message, intent.DocType, intent.EntityName)
		if err != nil {
			return nil, toolsCalled, fmt.Errorf("failed to process update query: %w", err)
		}
		emitTool("update_document", params)
		result, err := s.executeTool(ctx, "update_document", params)
		return result, toolsCalled, err

	case "delete":
		params, _ := json.Marshal(map[string]interface{}{
			"doctype": intent.DocType,
			"name":    intent.EntityName,
		})
		emitTool("delete_document", params)
		result, err := s.executeTool(ctx, "delete_document", params)
		return result, toolsCalled, err

	case "global_search", "search_all":
		p := map[string]interface{}{"text": message}
		if intent.DocType != "" {
			p["doctype"] = intent.DocType
		}
		params, _ := json.Marshal(p)
		emitTool("global_search", params)
		result, err := s.executeTool(ctx, "global_search", params)
		return result, toolsCalled, err

	case "aggregate":
		params, err := s.extractAggregationParams(ctx, message, intent.DocType)
		if err != nil {
			return nil, toolsCalled, fmt.Errorf("failed to process aggregation query: %w", err)
		}
		emitTool("aggregate_documents", params)
		result, err := s.executeTool(ctx, "aggregate_documents", params)
		return result, toolsCalled, err

	case "report":
		params, err := s.extractReportParams(ctx, message)
		if err != nil {
			return nil, toolsCalled, err // may be missing_params:...
		}
		emitTool("run_report", params)
		result, err := s.executeTool(ctx, "run_report", params)
		return result, toolsCalled, err
	}

	// Default: search-then-get or direct get/list.
	if intent.EntityName != "" && intent.RequiresSearch {
		searchRes, err := s.searchForEntity(ctx, intent.DocType, intent.EntityName)
		if err != nil {
			return nil, toolsCalled, fmt.Errorf("failed to find %s '%s': %w", intent.DocType, intent.EntityName, err)
		}
		if searchRes.EntityName == "" {
			return nil, toolsCalled, fmt.Errorf("no %s found matching '%s'", intent.DocType, intent.EntityName)
		}
		searchParams, _ := json.Marshal(map[string]interface{}{"doctype": intent.DocType, "name": searchRes.EntityName})
		emitTool("search_documents", searchParams)
		toolParams, _ := json.Marshal(map[string]interface{}{"doctype": intent.DocType, "name": searchRes.EntityName})
		emitTool(intent.Tool, toolParams)
		result, err := s.executeToolWithEntity(ctx, intent.Tool, intent.DocType, searchRes.EntityName)
		return result, toolsCalled, err
	}

	if intent.EntityName != "" {
		entityParams, _ := json.Marshal(map[string]interface{}{"doctype": intent.DocType, "name": intent.EntityName})
		emitTool(intent.Tool, entityParams)
		result, err := s.executeToolWithEntity(ctx, intent.Tool, intent.DocType, intent.EntityName)
		return result, toolsCalled, err
	}

	emitTool(intent.Tool, intent.Params)
	result, err := s.executeTool(ctx, intent.Tool, intent.Params)
	return result, toolsCalled, err
}

// streamFormatResponse builds an LLM prompt from the raw tool output and
// streams the tokens back over SSE.  Returns true if streaming succeeded.
func (s *MCPServer) streamFormatResponse(ctx context.Context, sw *sseWriter, userQuery, rawData string) bool {
	var streamer llm.Streamer

	// Prefer Manager (has fallback logic) if it implements Streamer.
	if s.llmManager != nil {
		if st, ok := interface{}(s.llmManager).(llm.Streamer); ok {
			streamer = st
		}
	}
	// Fallback to legacy client.
	if streamer == nil && s.llmClient != nil {
		if st, ok := s.llmClient.(llm.Streamer); ok {
			streamer = st
		}
	}
	if streamer == nil {
		return false
	}

	prompt := buildFormatPrompt(userQuery, rawData)
	tokenCh, err := streamer.GenerateStream(ctx, prompt)
	if err != nil {
		slog.Warn("GenerateStream failed, will use non-streaming fallback", "error", err)
		return false
	}

	for {
		select {
		case <-ctx.Done():
			slog.Debug("sse: client disconnected; stopping stream", "error", ctx.Err())
			return true
		case token, ok := <-tokenCh:
			if !ok {
				return true
			}
			sw.content(token)
		}
	}
}

// buildFormatPrompt constructs the same formatting prompt used by
// formatResponseWithLLM so the two paths produce consistent output.
func buildFormatPrompt(userQuery, rawData string) string {
	return fmt.Sprintf(`You are a data formatter that converts JSON data into user-requested formats.

CRITICAL RULES:
1. NEVER MAKE UP OR INVENT DATA - Only format what's actually in the raw data
2. If raw data contains an error message, show it clearly to the user
3. If raw data is empty or has no results, say so explicitly

User query: %s

Raw data from ERPNext:
%s

Format this data in a clear, readable way that answers the user's question.`, userQuery, rawData)
}

// dataQuality returns a quality label based on the length of the formatted text.
func dataQuality(text string) string {
	switch {
	case len(text) > 1000:
		return "high"
	case len(text) > 100:
		return "medium"
	default:
		return "low"
	}
}
