package notion

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	testTokenGET         = "Bearer test-token"
	testContentTypeGET   = "application/json"
	testNotionVersionGET = "2022-06-28"
	testMethodPOSTGET    = "POST"
)

//nolint:gocyclo // Test function with multiple test cases
func TestLoadTasks(t *testing.T) {
	tests := []struct {
		name           string
		dbID           string
		mockResponse   interface{}
		mockStatusCode int
		wantErr        bool
		wantTasks      int
		validateReq    func(*testing.T, *http.Request)
	}{
		{
			name: "successful load with one task",
			dbID: "db-123",
			mockResponse: map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id": "page-123",
						"properties": map[string]interface{}{
							"Задача": map[string]interface{}{
								"title": []map[string]interface{}{
									{"plain_text": "Test Task"},
								},
							},
							"Исполнитель": map[string]interface{}{
								"People": []map[string]interface{}{
									{"name": "Alice", "id": "user-1"},
								},
							},
							"Дедлайн": map[string]interface{}{
								"Date": map[string]interface{}{
									"start": "2025-12-31",
								},
							},
						},
					},
				},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantTasks:      1,
			validateReq: func(t *testing.T, req *http.Request) {
				if req.Method != testMethodPOSTGET {
					t.Errorf("expected method POST, got %s", req.Method)
				}
				if !strings.Contains(req.URL.Path, "databases/db-123/query") {
					t.Errorf("expected path to contain databases/db-123/query, got %s", req.URL.Path)
				}
				if req.Header.Get("Authorization") != testTokenGET {
					t.Errorf("expected Authorization header, got %s", req.Header.Get("Authorization"))
				}
				if req.Header.Get("Content-Type") != testContentTypeGET {
					t.Errorf("expected Content-Type header, got %s", req.Header.Get("Content-Type"))
				}
				if req.Header.Get("Notion-Version") != testNotionVersionGET {
					t.Errorf("expected Notion-Version header, got %s", req.Header.Get("Notion-Version"))
				}

				var payload loadPayload
				bodyBytes, _ := io.ReadAll(req.Body) //nolint:errcheck
				if err := json.Unmarshal(bodyBytes, &payload); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}
				if payload.Filter == nil {
					t.Error("expected filter in payload")
				}
				filter, ok := payload.Filter["and"].([]interface{})
				if !ok || len(filter) == 0 {
					t.Error("expected and filter with elements")
				}
			},
		},
		{
			name: "successful load with multiple tasks",
			dbID: "db-456",
			mockResponse: map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id": "page-1",
						"properties": map[string]interface{}{
							"Задача": map[string]interface{}{
								"title": []map[string]interface{}{
									{"plain_text": "Task 1"},
								},
							},
							"Исполнитель": map[string]interface{}{
								"People": []map[string]interface{}{
									{"name": "Alice", "id": "user-1"},
								},
							},
						},
					},
					{
						"id": "page-2",
						"properties": map[string]interface{}{
							"Задача": map[string]interface{}{
								"title": []map[string]interface{}{
									{"plain_text": "Task 2"},
								},
							},
							"Исполнитель": map[string]interface{}{
								"People": []map[string]interface{}{
									{"name": "Bob", "id": "user-2"},
								},
							},
						},
					},
				},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantTasks:      2,
		},
		{
			name: "API error response",
			dbID: "db-error",
			mockResponse: map[string]interface{}{
				"object": "error",
				"code":   "invalid_request",
			},
			mockStatusCode: http.StatusBadRequest,
			wantErr:        true,
		},
		{
			name: "task with invalid structure is skipped",
			dbID: "db-partial",
			mockResponse: map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id": "page-valid",
						"properties": map[string]interface{}{
							"Задача": map[string]interface{}{
								"title": []map[string]interface{}{
									{"plain_text": "Valid Task"},
								},
							},
							"Исполнитель": map[string]interface{}{
								"People": []map[string]interface{}{
									{"name": "Alice", "id": "user-1"},
								},
							},
						},
					},
					{
						"id":         "page-invalid",
						"properties": map[string]interface{}{},
					},
				},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantTasks:      1, // Invalid task should be skipped
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
					validateReq.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
					tt.validateReq(t, validateReq)
				}

				// Write response
				w.Header().Set("Content-Type", testContentTypeGET)
				w.WriteHeader(tt.mockStatusCode)
				_ = json.NewEncoder(w).Encode(tt.mockResponse) //nolint:errcheck
			}))
			defer server.Close()

			// Create Notion client with test server URL
			notion := NewNotion("test-token")
			notion.SetAPIBaseURL(server.URL + "/")

			// Execute
			tasks, err := notion.LoadTasks(tt.dbID)

			// Validate
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(tasks) != tt.wantTasks {
					t.Errorf("expected %d tasks, got %d", tt.wantTasks, len(tasks))
				}
				// Validate task structure for valid tasks
				if len(tasks) > 0 && !tt.wantErr {
					task := tasks[0]
					if task.Title == "" {
						t.Error("expected task to have title")
					}
					if len(task.Assignees) == 0 {
						t.Error("expected task to have assignees")
					}
					if !strings.HasPrefix(task.Link, notionURL) {
						t.Errorf("expected task link to start with %s, got %s", notionURL, task.Link)
					}
				}
			}
		})
	}
}

func TestParseTask_DeadlineParsing(t *testing.T) {
	tests := []struct {
		name           string
		dbID           string
		deadlineStart  string
		expectedTime   time.Time
		wantParseError bool
	}{
		{
			name:           "valid deadline date",
			dbID:           "db-1",
			deadlineStart:  "2025-12-31",
			expectedTime:   time.Date(2025, 12, 31, 0, 0, 0, 0, time.Now().Location()),
			wantParseError: false,
		},
		{
			name:           "empty deadline",
			dbID:           "db-2",
			deadlineStart:  "",
			expectedTime:   time.Time{},
			wantParseError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResponse := map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id": "page-123",
						"properties": map[string]interface{}{
							"Задача": map[string]interface{}{
								"title": []map[string]interface{}{
									{"plain_text": "Test Task"},
								},
							},
							"Исполнитель": map[string]interface{}{
								"People": []map[string]interface{}{
									{"name": "Alice", "id": "user-1"},
								},
							},
							"Дедлайн": map[string]interface{}{
								"Date": map[string]interface{}{
									"start": tt.deadlineStart,
								},
							},
						},
					},
				},
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", testContentTypeGET)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(mockResponse) //nolint:errcheck
			}))
			defer server.Close()

			notion := NewNotion("test-token")
			notion.SetAPIBaseURL(server.URL + "/")

			tasks, err := notion.LoadTasks(tt.dbID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(tasks) != 1 {
				t.Fatalf("expected 1 task, got %d", len(tasks))
			}

			if !tasks[0].Deadline.Equal(tt.expectedTime) &&
				!tasks[0].Deadline.IsZero() &&
				!tt.expectedTime.IsZero() {
				t.Errorf("expected deadline %v, got %v", tt.expectedTime, tasks[0].Deadline)
			}
		})
	}
}

func TestParseTask(t *testing.T) {
	tests := []struct {
		name       string
		input      loadResultEntry
		expected   Task
		wantErr    bool
		errMessage string
	}{
		{
			name: "valid task with one assignee",
			input: loadResultEntry{
				ID: "abc-def",
				Properties: loadResultProperty{
					"Задача": {
						Title: []struct {
							PlainText string `json:"plain_text"`
						}{{PlainText: "Single Assignee Task"}},
					},
					"Исполнитель": {
						People: []Assignee{{Name: "Alice", ID: "uuid-alice"}},
					},
					"Дедлайн": {
						Date: struct {
							Start string `json:"start"`
						}{Start: "2025-12-31"},
					},
				},
			},
			expected: Task{
				Title:     "Single Assignee Task",
				Assignees: []Assignee{{Name: "Alice", ID: "uuid-alice"}},
				Deadline:  mustParse("2025-12-31"),
				Link:      "https://www.notion.so/abcdef",
			},
		},
		{
			name: "valid task with multiple assignees",
			input: loadResultEntry{
				ID: "abc-xyz",
				Properties: loadResultProperty{
					"Задача": {
						Title: []struct {
							PlainText string `json:"plain_text"`
						}{{PlainText: "Multiple Assignees Task"}},
					},
					"Исполнитель": {
						People: []Assignee{
							{Name: "Alice", ID: "uuid-alice"},
							{Name: "Bob", ID: "uuid-bob"},
						},
					},
					"Дедлайн": {
						Date: struct {
							Start string `json:"start"`
						}{Start: "2025-11-30"},
					},
				},
			},
			expected: Task{
				Title: "Multiple Assignees Task",
				Assignees: []Assignee{
					{Name: "Alice", ID: "uuid-alice"},
					{Name: "Bob", ID: "uuid-bob"},
				},
				Deadline: mustParse("2025-11-30"),
				Link:     "https://www.notion.so/abcxyz",
			},
		},
		{
			name:       "missing title field",
			input:      loadResultEntry{ID: "id123", Properties: loadResultProperty{}},
			wantErr:    true,
			errMessage: "missing title",
		},
		{
			name: "missing assignee field",
			input: loadResultEntry{
				ID: "id999",
				Properties: loadResultProperty{
					"Задача": {
						Title: []struct {
							PlainText string `json:"plain_text"`
						}{{PlainText: "Task Without Assignee"}},
					},
				},
			},
			wantErr:    true,
			errMessage: "missing assignees",
		},
		{
			name: "invalid date format",
			input: loadResultEntry{
				ID: "id456",
				Properties: loadResultProperty{
					"Задача": {
						Title: []struct {
							PlainText string `json:"plain_text"`
						}{{PlainText: "Some task"}},
					},
					"Исполнитель": {
						People: []Assignee{{Name: "Alice", ID: "uuid-alice"}},
					},
					"Дедлайн": {
						Date: struct {
							Start string `json:"start"`
						}{Start: "not-a-date"},
					},
				},
			},
			expected: Task{
				Title:     "Some task",
				Assignees: []Assignee{{Name: "Alice", ID: "uuid-alice"}},
				Deadline:  time.Time{},
				Link:      "https://www.notion.so/id456",
			},
		},
		{
			name: "incorrectly formatted date MM-DD-YYYY",
			input: loadResultEntry{
				ID: "id789",
				Properties: loadResultProperty{
					"Задача": {
						Title: []struct {
							PlainText string `json:"plain_text"`
						}{{PlainText: "Another task"}},
					},
					"Исполнитель": {
						People: []Assignee{{Name: "Alice", ID: "uuid-alice"}},
					},
					"Дедлайн": {
						Date: struct {
							Start string `json:"start"`
						}{Start: "01-12-2025"},
					},
				},
			},
			expected: Task{
				Title:     "Another task",
				Assignees: []Assignee{{Name: "Alice", ID: "uuid-alice"}},
				Deadline:  time.Time{},
				Link:      "https://www.notion.so/id789",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := parseTask(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				if tt.errMessage != "" && !strings.Contains(err.Error(), tt.errMessage) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errMessage)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if task.Title != tt.expected.Title {
				t.Errorf("Title = %q, want %q", task.Title, tt.expected.Title)
			}
			if !reflect.DeepEqual(task.Assignees, tt.expected.Assignees) {
				t.Errorf("Assignees = %+v, want %+v", task.Assignees, tt.expected.Assignees)
			}
			if !task.Deadline.Equal(tt.expected.Deadline) {
				t.Errorf("Deadline = %v, want %v", task.Deadline, tt.expected.Deadline)
			}
			if task.Link != tt.expected.Link {
				t.Errorf("Link = %s, want %s", task.Link, tt.expected.Link)
			}
		})
	}
}

func mustParse(s string) time.Time {
	loc := time.Now().Location()
	tm, _ := time.ParseInLocation("2006-01-02", s, loc) //nolint:errcheck

	return tm
}
