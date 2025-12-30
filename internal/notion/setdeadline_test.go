package notion

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	testTokenDeadline         = "Bearer test-token"
	testContentTypeDeadline   = "application/json"
	testNotionVersionDeadline = "2022-06-28"
	testMethodPATCH           = "PATCH"
)

//nolint:gocyclo // Test function with multiple test cases
func TestSetDeadline(t *testing.T) {
	tests := []struct {
		name           string
		request        *SetDeadlineRequest
		mockResponse   interface{}
		mockStatusCode int
		wantErr        bool
		validateReq    func(*testing.T, *http.Request)
	}{
		{
			name: "successful deadline set",
			request: &SetDeadlineRequest{
				TaskLink: "https://www.notion.so/12345678901234567890123456789012",
				Deadline: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
			},
			mockResponse:   map[string]interface{}{"id": "page-123"},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			validateReq: func(t *testing.T, req *http.Request) {
				if req.Method != testMethodPATCH {
					t.Errorf("expected method PATCH, got %s", req.Method)
				}
				if !strings.Contains(req.URL.Path, "pages/") {
					t.Errorf("expected path to contain pages/, got %s", req.URL.Path)
				}
				if req.Header.Get("Authorization") != testTokenDeadline {
					t.Errorf("expected Authorization header, got %s", req.Header.Get("Authorization"))
				}
				if req.Header.Get("Content-Type") != testContentTypeDeadline {
					t.Errorf("expected Content-Type header, got %s", req.Header.Get("Content-Type"))
				}
				if req.Header.Get("Notion-Version") != testNotionVersionDeadline {
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

				deadlineField, ok := props["Дедлайн"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected Дедлайн property to be map")
				}

				dateField, ok := deadlineField["date"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected date field to be map")
				}

				startDate, ok := dateField["start"].(string)
				if !ok {
					t.Fatalf("expected start to be string")
				}

				if startDate != "2025-12-31" {
					t.Errorf("expected start date '2025-12-31', got %s", startDate)
				}
			},
		},
		{
			name: "invalid task link",
			request: &SetDeadlineRequest{
				TaskLink: "invalid-link",
				Deadline: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
			},
			wantErr: true,
		},
		{
			name: "API error response",
			request: &SetDeadlineRequest{
				TaskLink: "https://www.notion.so/12345678901234567890123456789012",
				Deadline: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
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
				w.Header().Set("Content-Type", testContentTypeDeadline)
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
			err := notion.SetDeadline(tt.request)

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

func TestExtractPageID(t *testing.T) {
	tests := []struct {
		name     string
		link     string
		expected string
	}{
		{
			name:     "standard notion URL",
			link:     "https://www.notion.so/12345678901234567890123456789012",
			expected: "12345678-9012-3456-7890-123456789012",
		},
		{
			name:     "notion URL with title",
			link:     "https://www.notion.so/Test-Page-12345678901234567890123456789012",
			expected: "12345678-9012-3456-7890-123456789012",
		},
		{
			name:     "notion URL with dashes in ID (will be reformatted)",
			link:     "https://www.notion.so/12345678901234567890123456789012",
			expected: "12345678-9012-3456-7890-123456789012",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPageID(tt.link)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
