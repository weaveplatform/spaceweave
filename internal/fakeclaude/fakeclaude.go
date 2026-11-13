// Package fakeclaude is a stand-in for the Anthropic Messages API that plays
// back scripted replies, so the tracer's agent loop can be tested end to end,
// through the real SDK, without a network or an API key. It speaks the
// streaming (server-sent events) form the tracer uses and the plain JSON
// form, and records every request so tests can inspect what was sent.
package fakeclaude

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/go-chi/chi/v5"
)

// Block is one content block of a scripted reply: Text for a text block, or
// Name and Input for a tool call.
type Block struct {
	Text  string
	ID    string
	Name  string
	Input any
	// RawInput, when set, is sent as the tool input verbatim, for testing
	// malformed or truncated JSON.
	RawInput string
}

// Reply is one scripted response. Status, when not 0 or 200, makes the
// request fail with that HTTP status instead.
type Reply struct {
	Content    []Block
	StopReason string
	Category   string // stop_details.category for a refusal
	Usage      Usage
	Status     int
}

// Usage is the token accounting a reply reports.
type Usage struct {
	Input, Output, CacheRead, CacheWrite int64
}

// Script decides the reply to the n-th request (from 0), given its body.
type Script func(n int, req Request) Reply

// Request is the decoded body of a Messages request.
type Request struct {
	Raw      json.RawMessage   `json:"-"`
	Model    string            `json:"model"`
	Stream   bool              `json:"stream"`
	Messages []json.RawMessage `json:"messages"`
	Header   http.Header       `json:"-"`
}

// Server is a running fake.
type Server struct {
	*httptest.Server

	mu       sync.Mutex
	requests []Request
	script   Script
}

// New starts a fake API that answers with script.
func New(script Script) *Server {
	s := &Server{script: script}
	r := chi.NewRouter()
	r.Post("/v1/messages", s.messages)
	s.Server = httptest.NewServer(r)
	return s
}

// Requests returns the requests received so far.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

func (s *Server) messages(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Raw = body
	req.Header = r.Header.Clone()
	s.mu.Lock()
	n := len(s.requests)
	s.requests = append(s.requests, req)
	s.mu.Unlock()

	reply := s.script(n, req)
	if reply.Status != 0 && reply.Status != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(reply.Status)
		_, _ = fmt.Fprintf(
			w,
			`{"type":"error","error":{"type":"api_error","message":"scripted failure %d"}}`,
			reply.Status,
		)
		return
	}
	if reply.StopReason == "" {
		reply.StopReason = "end_turn"
	}
	for i := range reply.Content {
		if reply.Content[i].Name != "" && reply.Content[i].ID == "" {
			reply.Content[i].ID = fmt.Sprintf("toolu_%d_%d", n, i)
		}
	}
	if req.Stream {
		stream(w, n, req.Model, reply)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(message(n, req.Model, reply, true))
}

func usage(u Usage, output int64) map[string]any {
	return map[string]any{
		"input_tokens":                u.Input,
		"output_tokens":               output,
		"cache_read_input_tokens":     u.CacheRead,
		"cache_creation_input_tokens": u.CacheWrite,
	}
}

func message(n int, model string, r Reply, complete bool) map[string]any {
	m := map[string]any{
		"id": fmt.Sprintf("msg_%d", n), "type": "message", "role": "assistant", "model": model,
		"content": []any{}, "stop_reason": nil, "stop_sequence": nil, "usage": usage(r.Usage, 0),
	}
	if complete {
		blocks := make([]any, 0, len(r.Content))
		for _, b := range r.Content {
			blocks = append(blocks, block(b, true))
		}
		m["content"] = blocks
		m["stop_reason"] = r.StopReason
		m["usage"] = usage(r.Usage, r.Usage.Output)
		if r.StopReason == "refusal" {
			m["stop_details"] = map[string]any{
				"type":        "refusal",
				"category":    r.Category,
				"explanation": "scripted refusal",
			}
		}
	}
	return m
}

func block(b Block, complete bool) map[string]any {
	if b.Name == "" {
		text := ""
		if complete {
			text = b.Text
		}
		return map[string]any{"type": "text", "text": text}
	}
	var input any = map[string]any{}
	if complete {
		input = b.Input
	}
	return map[string]any{"type": "tool_use", "id": b.ID, "name": b.Name, "input": input}
}

// stream writes the reply as the event sequence the real API sends.
func stream(w http.ResponseWriter, n int, model string, r Reply) {
	w.Header().Set("Content-Type", "text/event-stream")
	send := func(event string, data any) {
		b, _ := json.Marshal(data)
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
	}
	send(
		"message_start",
		map[string]any{"type": "message_start", "message": message(n, model, r, false)},
	)
	for i, b := range r.Content {
		send(
			"content_block_start",
			map[string]any{
				"type":          "content_block_start",
				"index":         i,
				"content_block": block(b, false),
			},
		)
		var delta map[string]any
		if b.Name == "" {
			delta = map[string]any{"type": "text_delta", "text": b.Text}
		} else {
			raw := b.RawInput
			if raw == "" {
				j, _ := json.Marshal(b.Input)
				raw = string(j)
			}
			delta = map[string]any{"type": "input_json_delta", "partial_json": raw}
		}
		send(
			"content_block_delta",
			map[string]any{"type": "content_block_delta", "index": i, "delta": delta},
		)
		send("content_block_stop", map[string]any{"type": "content_block_stop", "index": i})
	}
	md := map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": r.StopReason, "stop_sequence": nil},
		"usage": usage(r.Usage, r.Usage.Output),
	}
	if r.StopReason == "refusal" {
		md["delta"].(map[string]any)["stop_details"] = map[string]any{
			"type":        "refusal",
			"category":    r.Category,
			"explanation": "scripted refusal",
		}
	}
	send("message_delta", md)
	send("message_stop", map[string]any{"type": "message_stop"})
}
