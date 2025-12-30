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
	testTokenStatus         = "Bearer test-token"
	testContentTypeStatus   = "application/json"
	testNotionVersionStatus = "2022-06-28"
	testMethodPATCHStatus   = "PATCH"
)

//nolint:gocyclo // Test function with multiple test cases
func TestSetStatus(t *testing.T) {
	tests := []struct {
		name           string
		request        *SetStatusRequest
		mockResponse   interface{}
		mockStatusCode int
		wantErr        bool
		validateReq    func(*testing.T, *http.Request)
	}{
		{
			name: "successful status set",
			request: &SetStatusRequest{
				TaskLink: "https://www.notion.so/12345678901234567890123456789012",
				Status:   StatusDone,
			},
			mockResponse:   map[string]interface{}{"id": "page-123"},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			validateReq: func(t *testing.T, req *http.Request) {
				if req.Method != testMethodPATCHStatus {
					t.Errorf("expected method PATCH, got %s", req.Method)
				}
				if !strings.Contains(req.URL.Path, "pages/") {
					t.Errorf("expected path to contain pages/, got %s", req.URL.Path)
				}
				if req.Header.Get("Authorization") != testTokenStatus {
					t.Errorf("expected Authorization header, got %s", req.Header.Get("Authorization"))
				}
				if req.Header.Get("Content-Type") != testContentTypeStatus {
					t.Errorf("expected Content-Type header, got %s", req.Header.Get("Content-Type"))
				}
				if req.Header.Get("Notion-Version") != testNotionVersionStatus {
					t.Errorf("expected Notion-Version header, got %s", req.Header.Get("Notion-Version"))
				}

				var payload map[string]interface{}
				bodyBytes, _ := io.ReadAll(req.Body) //nolint:errcheck
				if err := json.Unmarshal(bodyBytes, &payload); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				props, ok := payload["properties"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected properties to be map")
				}

				statusField, ok := props["Статус"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected Статус property to be map")
				}

				selectField, ok := statusField["select"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected select field to be map")
				}

				statusName, ok := selectField["name"].(string)
				if !ok {
					t.Fatalf("expected name to be string")
				}

				if statusName != StatusDone {
					t.Errorf("expected status '%s', got %s", StatusDone, statusName)
				}
			},
		},
		{
			name: "set status to new",
			request: &SetStatusRequest{
				TaskLink: "https://www.notion.so/12345678901234567890123456789012",
				Status:   StatusNew,
			},
			mockResponse:   map[string]interface{}{"id": "page-123"},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			validateReq: func(t *testing.T, req *http.Request) {
				var payload map[string]interface{}
				bodyBytes, _ := io.ReadAll(req.Body) //nolint:errcheck
				if err := json.Unmarshal(bodyBytes, &payload); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				props := payload["properties"].(map[string]interface{})
				statusField := props["Статус"].(map[string]interface{})
				selectField := statusField["select"].(map[string]interface{})
				statusName := selectField["name"].(string)

				if statusName != StatusNew {
					t.Errorf("expected status '%s', got %s", StatusNew, statusName)
				}
			},
		},
		{
			name: "invalid task link",
			request: &SetStatusRequest{
				TaskLink: "invalid-link",
				Status:   StatusDone,
			},
			wantErr: true,
		},
		{
			name: "API error response",
			request: &SetStatusRequest{
				TaskLink: "https://www.notion.so/12345678901234567890123456789012",
				Status:   StatusDone,
			},
			mockResponse: map[string]interface{}{
				"object": "error",
				"code":   "invalid_request",
			},
			mockStatusCode: http.StatusBadRequest,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				// Validate request
				if tt.validateReq != nil {
					bodyBytes, _ := io.ReadAll(req.Body) //nolint:errcheck
					validateReq := req.Clone(req.Context())
					validateReq.Body = io.NopCloser(bytes.NewReader(bodyBytes))
					tt.validateReq(t, validateReq)
				}

				// Write response
				w.Header().Set("Content-Type", testContentTypeStatus)
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					_ = json.NewEncoder(w).Encode(tt.mockResponse) //nolint:errcheck
				}
			}))
			defer server.Close()

			// Create Notion client with test server URL
			notion := NewNotion("test-token")
			notion.SetAPIBaseURL(server.URL + "/")

			// Execute
			err := notion.SetStatus(tt.request)

			// Validate
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
