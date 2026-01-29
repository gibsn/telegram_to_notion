package notion

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testTokenRetries         = "Bearer test-token"
	testContentTypeRetries   = "application/json"
	testNotionVersionRetries = "2022-06-28"
)

type capturedReq struct {
	Method        string
	Path          string
	Query         string
	Headers       http.Header
	Body          []byte
	ContentLength int64
}

func TestDoWithRetries_ReplaysIdenticalRequest(t *testing.T) { //nolint:gocyclo
	tests := []struct {
		name        string
		method      string
		path        string
		body        []byte
		failFirstN  int
		wantCalls   int
		validateReq func(*testing.T, *capturedReq)
	}{
		{
			name:   "retries re-send identical POST with same body and headers",
			method: http.MethodPost,
			path:   "/pages",
			body:   []byte(`{"hello":"world","n":1}`),
			// one failed attempt + one success attempt
			failFirstN: 1,
			wantCalls:  2,
			validateReq: func(t *testing.T, r *capturedReq) {
				if r.Method != http.MethodPost {
					t.Errorf("expected method %s, got %s", http.MethodPost, r.Method)
				}
				if !strings.HasSuffix(r.Path, "/pages") {
					t.Errorf("expected path to end with /pages, got %s", r.Path)
				}
				if r.Headers.Get("Authorization") != testTokenRetries {
					t.Errorf(
						"expected Authorization header %q, got %q", testTokenRetries,
						r.Headers.Get("Authorization"),
					)
				}
				if r.Headers.Get("Content-Type") != testContentTypeRetries {
					t.Errorf(
						"expected Content-Type %q, got %q", testContentTypeRetries,
						r.Headers.Get("Content-Type"),
					)
				}
				if r.Headers.Get("Accept") != testContentTypeRetries {
					t.Errorf(
						"expected Accept %q, got %q", testContentTypeRetries,
						r.Headers.Get("Accept"),
					)
				}
				if r.Headers.Get("Notion-Version") != testNotionVersionRetries {
					t.Errorf(
						"expected Notion-Version %q, got %q", testNotionVersionRetries,
						r.Headers.Get("Notion-Version"),
					)
				}
			},
		},
		{
			name:       "retries re-send identical PATCH with same body and headers",
			method:     http.MethodPatch,
			path:       "/pages/page-123",
			body:       []byte(`{"properties":{"X":{"number":1}}}`),
			failFirstN: 1,
			wantCalls:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				callN    int
				captured []capturedReq
			)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				callN++

				// Capture the incoming request (including body) for comparison
				bodyBytes, _ := io.ReadAll(req.Body) //nolint:errcheck
				_ = req.Body.Close()

				captured = append(captured, capturedReq{
					Method:        req.Method,
					Path:          req.URL.Path,
					Query:         req.URL.RawQuery,
					Headers:       req.Header.Clone(),
					Body:          bodyBytes,
					ContentLength: req.ContentLength,
				})

				// Fail first N calls, succeed after
				if callN <= tt.failFirstN {
					w.Header().Set("Content-Type", testContentTypeRetries)
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode( //nolint:errcheck
						map[string]any{"object": "error", "code": "bad_request"},
					) //nolint:errcheck
					return
				}

				w.Header().Set("Content-Type", testContentTypeRetries)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true}) //nolint:errcheck
			}))
			defer server.Close()

			// Client under test
			notion := NewNotion("test-token")
			notion.SetAPIBaseURL(server.URL + "/")

			req, err := http.NewRequest(
				tt.method, notion.apiBaseURL+strings.TrimPrefix(tt.path, "/"), nil,
			)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			resp, err := notion.doWithRetries(req, tt.body)
			if err != nil {
				t.Fatalf("expected success, got error: %v", err)
			}
			defer resp.Body.Close()

			if callN != tt.wantCalls {
				t.Fatalf("expected %d calls, got %d", tt.wantCalls, callN)
			}
			if len(captured) != tt.wantCalls {
				t.Fatalf("expected %d captured requests, got %d", tt.wantCalls, len(captured))
			}

			// Validate each captured request
			for i := range captured {
				if tt.validateReq != nil {
					tt.validateReq(t, &captured[i])
				}
				// Body must be exactly what we passed to doWithRetries
				if !bytes.Equal(captured[i].Body, tt.body) {
					t.Errorf(
						"attempt %d: request body differs\nwant: %s\ngot:  %s", i+1,
						string(tt.body), string(captured[i].Body),
					)
				}
			}

			// All attempts must be identical (method/path/query/headers/body/content-length)
			base := captured[0]
			for i := 1; i < len(captured); i++ {
				cur := captured[i]

				if cur.Method != base.Method {
					t.Errorf("attempt %d: method differs: %s vs %s", i+1, cur.Method, base.Method)
				}
				if cur.Path != base.Path {
					t.Errorf("attempt %d: path differs: %s vs %s", i+1, cur.Path, base.Path)
				}
				if cur.Query != base.Query {
					t.Errorf("attempt %d: query differs: %s vs %s", i+1, cur.Query, base.Query)
				}

				// Compare only the headers we care about (avoid flaky defaults like User-Agent, etc.)
				for _, h := range []string{
					"Authorization", "Content-Type", "Accept", "Notion-Version",
				} {
					if cur.Headers.Get(h) != base.Headers.Get(h) {
						t.Errorf(
							"attempt %d: header %s differs: %q vs %q", i+1, h,
							cur.Headers.Get(h), base.Headers.Get(h),
						)
					}
				}

				if !bytes.Equal(cur.Body, base.Body) {
					t.Errorf("attempt %d: body differs", i+1)
				}

				// If ты добавил req.ContentLength = int64(len(body)) — этот ассерт начнет быть полезным.
				// Сейчас он может быть -1 в зависимости от реализации.
				_ = cur.ContentLength
			}
		})
	}
}
