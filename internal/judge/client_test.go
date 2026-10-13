package judge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestClientSendsSchemaAndReadsEitherField(t *testing.T) {
	var got map[string]any
	reply := `{"choices":[{"message":{"content":"","reasoning_content":" {\"results\":[]} "}}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer key" {
			t.Errorf("request %s, auth %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, reply)
	}))
	defer srv.Close()

	c := NewClient(srv.URL+"/v1/", "m", "key")
	schema := json.RawMessage(`{"type":"object"}`)
	// With a schema LM Studio leaves content empty and answers in reasoning_content.
	text, err := c.Complete(context.Background(), "sys", "user", schema, 123)
	if err != nil || text != `{"results":[]}` {
		t.Fatalf("answer %q, %v", text, err)
	}
	format, _ := got["response_format"].(map[string]any)
	js, _ := format["json_schema"].(map[string]any)
	if format["type"] != "json_schema" || js["strict"] != true || js["schema"] == nil || got["max_tokens"] != float64(123) || got["model"] != "m" {
		t.Errorf("request body %v", got)
	}

	reply = `{"choices":[{"message":{"content":"{\"a\":1}","reasoning_content":"thinking"}}]}`
	if text, err = c.Complete(context.Background(), "sys", "user", schema, 1); err != nil || text != `{"a":1}` {
		t.Errorf("content should win: %q, %v", text, err)
	}
	for _, bad := range []string{`{"choices":[{"message":{"content":""},"finish_reason":"length"}]}`, `{"choices":[]}`, `{"error":{"message":"no model"}}`, `nope`} {
		reply = bad
		if _, err = c.Complete(context.Background(), "sys", "user", schema, 1); err == nil {
			t.Errorf("reply %s was accepted", bad)
		}
	}
}

// The model sits on a private address: a proxy from the environment must not
// be used for it, or the request never arrives.
func TestLocalModelBypassesProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://proxy.example:8080")
	t.Setenv("NO_PROXY", "")
	for host, direct := range map[string]bool{"192.168.50.182:1234": true, "127.0.0.1:1234": true, "localhost:1234": true, "10.0.0.5": true} {
		u, _ := url.Parse("http://" + host + "/v1")
		p, err := proxyUnlessLocal(&http.Request{URL: u})
		if err != nil || (p == nil) != direct {
			t.Errorf("%s: proxy %v, %v", host, p, err)
		}
	}
	if !strings.HasPrefix(NewClient("http://x/v1/", "m", "").BaseURL, "http://x/v1") || strings.HasSuffix(NewClient("http://x/v1/", "m", "").BaseURL, "/") {
		t.Error("base URL not normalised")
	}
}
