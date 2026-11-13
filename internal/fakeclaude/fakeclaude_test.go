package fakeclaude

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func post(t *testing.T, s *Server, body string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Post(s.URL+"/v1/messages", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, string(b)
}

func TestPlainJSONReply(t *testing.T) {
	s := New(func(n int, req Request) Reply {
		if req.Model != "m" || n != 0 {
			t.Errorf("request %d model %q", n, req.Model)
		}
		return Reply{
			Content:    []Block{{Text: "hi"}, {Name: "look", Input: map[string]int{"x": 1}}},
			StopReason: "refusal", Category: "cyber",
			Usage: Usage{Input: 3, Output: 4, CacheRead: 5, CacheWrite: 6},
		}
	})
	defer s.Close()
	resp, body := post(t, s, `{"model":"m","messages":[{"role":"user","content":"x"}]}`)
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Fatal(resp.Header)
	}
	var m struct {
		Content []struct {
			Type, Text, ID, Name string
			Input                map[string]int
		}
		StopReason  string `json:"stop_reason"`
		StopDetails struct {
			Category string
		} `json:"stop_details"`
		Usage map[string]int
	}
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatal(err)
	}
	if m.Content[0].Text != "hi" || m.Content[1].ID != "toolu_0_1" || m.Content[1].Input["x"] != 1 ||
		m.StopReason != "refusal" || m.StopDetails.Category != "cyber" || m.Usage["output_tokens"] != 4 ||
		m.Usage["cache_creation_input_tokens"] != 6 {
		t.Fatal(body)
	}
	reqs := s.Requests()
	if len(reqs) != 1 || len(reqs[0].Messages) != 1 || !bytes.Contains(reqs[0].Raw, []byte(`"model":"m"`)) {
		t.Fatalf("%+v", reqs)
	}
}

func TestStreamReply(t *testing.T) {
	s := New(func(int, Request) Reply {
		return Reply{Content: []Block{{Text: "a"}, {Name: "t", ID: "given", RawInput: `{"cut`}}}
	})
	defer s.Close()
	resp, body := post(t, s, `{"model":"m","stream":true,"messages":[]}`)
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal(resp.Header)
	}
	for _, want := range []string{
		"event: message_start", `{"text":"a","type":"text_delta"}`, `"id":"given"`, `"partial_json":"{\"cut"`,
		`"stop_reason":"end_turn"`, "event: message_stop",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("stream lacks %s:\n%s", want, body)
		}
	}
}

func TestStreamRefusal(t *testing.T) {
	s := New(func(int, Request) Reply { return Reply{StopReason: "refusal", Category: "bio"} })
	defer s.Close()
	_, body := post(t, s, `{"model":"m","stream":true}`)
	if !strings.Contains(body, `"category":"bio"`) {
		t.Fatal(body)
	}
}

func TestScriptedFailureAndBadRequests(t *testing.T) {
	s := New(func(int, Request) Reply { return Reply{Status: http.StatusTooManyRequests} })
	defer s.Close()
	resp, body := post(t, s, `{"model":"m"}`)
	if resp.StatusCode != http.StatusTooManyRequests || !strings.Contains(body, "scripted failure 429") {
		t.Fatal(resp.StatusCode, body)
	}
	resp, _ = post(t, s, `not json`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatal(resp.StatusCode)
	}
	if len(s.Requests()) != 1 {
		t.Fatal("an undecodable request is not recorded")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestUnreadableBody(t *testing.T) {
	s := New(func(int, Request) Reply { return Reply{} })
	defer s.Close()
	req, err := http.NewRequest(http.MethodPost, "/v1/messages", failingReader{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.messages(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatal(rec.Code)
	}
}
