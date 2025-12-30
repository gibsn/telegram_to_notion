package notion

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
)

const (
	testTokenTweaks         = "Bearer test-token"
	testContentTypeTweaks   = "application/json"
	testNotionVersionTweaks = "2022-06-28"
	testMethodPOSTTweaks    = "POST"
)

func TestLoadTracks(t *testing.T) {
	tests := []struct {
		name           string
		dbID           string
		mockResponse   interface{}
		mockStatusCode int
		wantErr        bool
		wantCount      int
		validateReq    func(*testing.T, *http.Request)
	}{
		{
			name: "successful load with one track",
			dbID: "db-tracks-123",
			mockResponse: map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id": "track-1",
						"properties": map[string]interface{}{
							"Название": map[string]interface{}{
								"title": []map[string]interface{}{
									{"plain_text": "Track One"},
								},
							},
						},
					},
				},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantCount:      1,
			validateReq: func(t *testing.T, req *http.Request) {
				if req.Method != testMethodPOSTTweaks {
					t.Errorf("expected method POST, got %s", req.Method)
				}
				expectedPath := path.Join("databases", "db-tracks-123", "query")
				if !strings.Contains(req.URL.Path, expectedPath) {
					t.Errorf("expected path to contain %s, got %s", expectedPath, req.URL.Path)
				}
				if req.Header.Get("Authorization") != testTokenTweaks {
					t.Errorf("expected Authorization header, got %s", req.Header.Get("Authorization"))
				}
				if req.Header.Get("Notion-Version") != testNotionVersionTweaks {
					t.Errorf("expected Notion-Version header, got %s", req.Header.Get("Notion-Version"))
				}
			},
		},
		{
			name: "successful load with multiple tracks",
			dbID: "db-tracks-456",
			mockResponse: map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id": "track-1",
						"properties": map[string]interface{}{
							"Название": map[string]interface{}{
								"title": []map[string]interface{}{
									{"plain_text": "Track One"},
								},
							},
						},
					},
					{
						"id": "track-2",
						"properties": map[string]interface{}{
							"Название": map[string]interface{}{
								"title": []map[string]interface{}{
									{"plain_text": "Track Two"},
								},
							},
						},
					},
				},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantCount:      2,
		},
		{
			name: "tracks without title are skipped",
			dbID: "db-tracks-partial",
			mockResponse: map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id": "track-valid",
						"properties": map[string]interface{}{
							"Название": map[string]interface{}{
								"title": []map[string]interface{}{
									{"plain_text": "Valid Track"},
								},
							},
						},
					},
					{
						"id":         "track-invalid",
						"properties": map[string]interface{}{},
					},
				},
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			wantCount:      1,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				// Validate request
				if tt.validateReq != nil {
					tt.validateReq(t, req)
				}

				// Write response
				w.Header().Set("Content-Type", testContentTypeTweaks)
				w.WriteHeader(tt.mockStatusCode)
				_ = json.NewEncoder(w).Encode(tt.mockResponse) //nolint:errcheck
			}))
			defer server.Close()

			// Create Notion client with test server URL
			notion := NewNotion("test-token")
			notion.SetAPIBaseURL(server.URL + "/")

			// Execute
			tracks, err := notion.LoadTracks(tt.dbID)

			// Validate
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(tracks) != tt.wantCount {
					t.Errorf("expected %d tracks, got %d", tt.wantCount, len(tracks))
				}
			}
		})
	}
}

//nolint:dupl,gocyclo // Test functions have similar structure but test different endpoints
func TestCreateTweakDemo(t *testing.T) {
	tests := []struct {
		name           string
		request        *CreateTweakRequest
		mockResponse   interface{}
		mockStatusCode int
		wantErr        bool
		validateReq    func(*testing.T, *http.Request)
	}{
		{
			name: "successful tweak creation",
			request: &CreateTweakRequest{
				Title:            "Test Tweak",
				TrackPageID:      "track-123",
				Start:            "0:00",
				End:              "1:00",
				Explanation:      "Test explanation",
				AuthorNotionUser: "user-123",
			},
			mockResponse: map[string]interface{}{
				"id": "12345678-1234-1234-1234-123456789abc",
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			validateReq: func(t *testing.T, req *http.Request) {
				if req.Method != testMethodPOSTTweaks {
					t.Errorf("expected method POST, got %s", req.Method)
				}
				if !strings.HasSuffix(req.URL.Path, "/pages") {
					t.Errorf("expected path to end with /pages, got %s", req.URL.Path)
				}

				var payload map[string]interface{}
				bodyBytes, _ := io.ReadAll(req.Body) //nolint:errcheck
				if err := json.Unmarshal(bodyBytes, &payload); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				props := payload["properties"].(map[string]interface{})

				// Check title
				titleField := props["Кратко"].(map[string]interface{})
				titleArray := titleField["title"].([]interface{})
				titleObj := titleArray[0].(map[string]interface{})
				textObj := titleObj["text"].(map[string]interface{})
				if textObj["content"] != "Test Tweak" {
					t.Errorf("expected title 'Test Tweak', got %v", textObj["content"])
				}

				// Check status (should be select type with TweakDemoStatusTODO)
				statusField := props["Статус"].(map[string]interface{})
				if _, ok := statusField["select"]; !ok {
					t.Error("expected status to be select type")
				}
				selectField := statusField["select"].(map[string]interface{})
				if selectField["name"] != TweakDemoStatusTODO {
					t.Errorf("expected status '%s', got %v", TweakDemoStatusTODO, selectField["name"])
				}

				// Check explanation
				explanationField := props["Пояснение"].(map[string]interface{})
				explanationArray := explanationField["rich_text"].([]interface{})
				explanationObj := explanationArray[0].(map[string]interface{})
				explanationText := explanationObj["text"].(map[string]interface{})
				if explanationText["content"] != "Test explanation" {
					t.Errorf("expected explanation 'Test explanation', got %v", explanationText["content"])
				}

				// Check track relation
				trackField := props["Песня"].(map[string]interface{})
				trackArray := trackField["relation"].([]interface{})
				trackObj := trackArray[0].(map[string]interface{})
				if trackObj["id"] != "track-123" {
					t.Errorf("expected track ID 'track-123', got %v", trackObj["id"])
				}

				// Check author
				authorField := props["Автор (Manual)"].(map[string]interface{})
				authorArray := authorField["people"].([]interface{})
				authorObj := authorArray[0].(map[string]interface{})
				if authorObj["id"] != "user-123" {
					t.Errorf("expected author ID 'user-123', got %v", authorObj["id"])
				}
			},
		},
		{
			name: "tweak without optional fields",
			request: &CreateTweakRequest{
				Title: "Simple Tweak",
			},
			mockResponse: map[string]interface{}{
				"id": "87654321-4321-4321-4321-cba987654321",
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			validateReq: func(t *testing.T, req *http.Request) {
				var payload map[string]interface{}
				bodyBytes, _ := io.ReadAll(req.Body) //nolint:errcheck
				if err := json.Unmarshal(bodyBytes, &payload); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				props := payload["properties"].(map[string]interface{})

				// Should not have optional fields
				if _, ok := props["Пояснение"]; ok {
					t.Error("expected no Пояснение property when not provided")
				}
				if _, ok := props["Песня"]; ok {
					t.Error("expected no Песня property when not provided")
				}
			},
		},
		{
			name:           "demo DB ID not set",
			request:        &CreateTweakRequest{Title: "Test"},
			mockStatusCode: http.StatusOK,
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
				w.Header().Set("Content-Type", testContentTypeTweaks)
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					_ = json.NewEncoder(w).Encode(tt.mockResponse) //nolint:errcheck
				}
			}))
			defer server.Close()

			// Create Notion client with test server URL
			notion := NewNotion("test-token")
			notion.SetAPIBaseURL(server.URL + "/")

			// Set DB IDs for valid test cases
			if !tt.wantErr || tt.name != "demo DB ID not set" {
				notion.SetTweaksDBIDs("demo-db-id", "mix-db-id")
			}

			// Execute
			url, err := notion.CreateTweakDemo(tt.request)

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

//nolint:dupl // Test functions have similar structure but test different endpoints
func TestCreateTweakMix(t *testing.T) {
	tests := []struct {
		name           string
		request        *CreateTweakRequest
		mockResponse   interface{}
		mockStatusCode int
		wantErr        bool
		validateReq    func(*testing.T, *http.Request)
	}{
		{
			name: "successful mix tweak creation",
			request: &CreateTweakRequest{
				Title:       "Mix Tweak",
				TrackPageID: "track-456",
				Explanation: "Mix explanation",
			},
			mockResponse: map[string]interface{}{
				"id": "abcdef12-3456-7890-abcd-ef1234567890",
			},
			mockStatusCode: http.StatusOK,
			wantErr:        false,
			validateReq: func(t *testing.T, req *http.Request) {
				var payload map[string]interface{}
				bodyBytes, _ := io.ReadAll(req.Body) //nolint:errcheck
				if err := json.Unmarshal(bodyBytes, &payload); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				props := payload["properties"].(map[string]interface{})

				// Check status (should be status type with TweakMixStatusAnalysis)
				statusField := props["Статус"].(map[string]interface{})
				if _, ok := statusField["status"]; !ok {
					t.Error("expected status to be status type")
				}
				statusTypeField := statusField["status"].(map[string]interface{})
				if statusTypeField["name"] != TweakMixStatusAnalysis {
					t.Errorf("expected status '%s', got %v", TweakMixStatusAnalysis, statusTypeField["name"])
				}
			},
		},
		{
			name:           "mix DB ID not set",
			request:        &CreateTweakRequest{Title: "Test"},
			mockStatusCode: http.StatusOK,
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
				w.Header().Set("Content-Type", testContentTypeTweaks)
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockResponse != nil {
					_ = json.NewEncoder(w).Encode(tt.mockResponse) //nolint:errcheck
				}
			}))
			defer server.Close()

			// Create Notion client with test server URL
			notion := NewNotion("test-token")
			notion.SetAPIBaseURL(server.URL + "/")

			// Set DB IDs for valid test cases
			if !tt.wantErr || tt.name != "mix DB ID not set" {
				notion.SetTweaksDBIDs("demo-db-id", "mix-db-id")
			}

			// Execute
			url, err := notion.CreateTweakMix(tt.request)

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
