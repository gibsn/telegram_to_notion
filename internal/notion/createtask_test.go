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
	testToken         = "Bearer test-token"
	testContentType   = "application/json"
	testNotionVersion = "2022-06-28"
	testMethodPOST    = "POST"
)

//nolint:gocyclo // Test function with multiple test cases
func TestCreateNotionTask(t *testing.T) {
	tests := []struct {
		name           string
		request        *CreateTaskRequest
		mockResponse   interface{}
		mockStatusCode int
		wantErr        bool
		validateReq    func(*testing.T, *http.Request)
	}{
		{
			name: "successful task creation",
			request: &CreateTaskRequest{
				NotionDBID:  "db-id-123",
				TaskName:    "Test Task",
				Assignees:   []string{"user-id-1"},
				Description: "Test description",
			},
			mockResponse: map[string]interface{}{
				"id": "12345678-1234-1234-1234-123456789abc",
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			validateReq: func(t *testing.T, req *http.Request) {
				if req.Method != testMethodPOST {
					t.Errorf("expected method POST, got %s", req.Method)
				}
				if !strings.HasSuffix(req.URL.Path, "/pages") {
					t.Errorf("expected path to end with /pages, got %s", req.URL.Path)
				}
				if req.Header.Get("Authorization") != testToken {
					t.Errorf("expected Authorization header, got %s", req.Header.Get("Authorization"))
				}
				if req.Header.Get("Content-Type") != testContentType {
					t.Errorf("expected Content-Type header, got %s", req.Header.Get("Content-Type"))
				}
				if req.Header.Get("Notion-Version") != testNotionVersion {
					t.Errorf("expected Notion-Version header, got %s", req.Header.Get("Notion-Version"))
				}

				var payload createPayload
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}
				if payload.Parent.DatabaseID != "db-id-123" {
					t.Errorf("expected database ID db-id-123, got %s", payload.Parent.DatabaseID)
				}

				taskNameField, ok := payload.Properties["Задача"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected Задача property to be map")
				}
				titleArray, ok := taskNameField["title"].([]interface{})
				if !ok || len(titleArray) == 0 {
					t.Fatalf("expected title array with at least one element")
				}
				titleObj, ok := titleArray[0].(map[string]interface{})
				if !ok {
					t.Fatalf("expected title object to be map")
				}
				textObj, ok := titleObj["text"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected text object to be map")
				}
				if textObj["content"] != "Test Task" {
					t.Errorf("expected task name 'Test Task', got %v", textObj["content"])
				}

				statusField, ok := payload.Properties["Статус"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected Статус property to be map")
				}
				selectField, ok := statusField["select"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected select field to be map")
				}
				if selectField["name"] != StatusNew {
					t.Errorf("expected status '%s', got %v", StatusNew, selectField["name"])
				}

				assigneeField, ok := payload.Properties["Исполнитель"].(map[string]interface{})
				if !ok {
					t.Fatalf("expected Исполнитель property to be map")
				}
				peopleArray, ok := assigneeField["people"].([]interface{})
				if !ok || len(peopleArray) != 1 {
					t.Fatalf("expected people array with 1 element, got %d", len(peopleArray))
				}
				personObj, ok := peopleArray[0].(map[string]interface{})
				if !ok {
					t.Fatalf("expected person object to be map")
				}
				if personObj["id"] != "user-id-1" {
					t.Errorf("expected assignee ID 'user-id-1', got %v", personObj["id"])
				}

				if len(payload.Children) != 1 {
					t.Fatalf("expected 1 child, got %d", len(payload.Children))
				}
				child := payload.Children[0]
				if child["type"] != "paragraph" {
					t.Errorf("expected child type 'paragraph', got %v", child["type"])
				}
			},
		},
		{
			name: "successful task creation without description",
			request: &CreateTaskRequest{
				NotionDBID: "db-id-456",
				TaskName:   "Simple Task",
			},
			mockResponse: map[string]interface{}{
				"id": "87654321-4321-4321-4321-cba987654321",
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			validateReq: func(t *testing.T, req *http.Request) {
				var payload createPayload
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}
				if len(payload.Children) != 0 {
					t.Errorf("expected no children, got %d", len(payload.Children))
				}
			},
		},
		{
			name: "successful task creation without assignees",
			request: &CreateTaskRequest{
				NotionDBID:  "db-id-789",
				TaskName:    "Unassigned Task",
				Description: "No assignees",
			},
			mockResponse: map[string]interface{}{
				"id": "abcdef12-3456-7890-abcd-ef1234567890",
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			validateReq: func(t *testing.T, req *http.Request) {
				var payload createPayload
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}
				if _, ok := payload.Properties["Исполнитель"]; ok {
					t.Error("expected no Исполнитель property when no assignees provided")
				}
			},
		},
		{
			name: "API error response",
			request: &CreateTaskRequest{
				NotionDBID: "db-id-error",
				TaskName:   "Error Task",
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
				// Read body once and restore it
				var bodyBytes []byte
				if tt.validateReq != nil {
					var err error
					bodyBytes, err = io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("failed to read request body: %v", err)
					}
					req.Body.Close()
					req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}

				// Validate request
				if tt.validateReq != nil {
					// Create a new request with the body for validation
					validateReq := req.Clone(req.Context())
					validateReq.Body = io.NopCloser(bytes.NewReader(bodyBytes))
					tt.validateReq(t, validateReq)
				}

				// Write response
				w.Header().Set("Content-Type", testContentType)
				w.WriteHeader(tt.mockStatusCode)
				_ = json.NewEncoder(w).Encode(tt.mockResponse) //nolint:errcheck
			}))
			defer server.Close()

			// Create Notion client with test server URL
			notion := NewNotion("test-token")
			notion.SetAPIBaseURL(server.URL + "/")

			// Execute
			url, err := notion.CreateNotionTask(tt.request)

			// Validate
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if !strings.HasPrefix(url, notionURL) {
					t.Errorf("expected URL to start with %s, got %s", notionURL, url)
				}
			}
		})
	}
}
