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

func TestLoadTracks(t *testing.T) { //nolint:gocyclo
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

				// Validate that filter is present in request body
				var payload map[string]interface{}
				bodyBytes, _ := io.ReadAll(req.Body) //nolint:errcheck
				if err := json.Unmarshal(bodyBytes, &payload); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				filter, ok := payload["filter"]
				if !ok {
					t.Error("expected filter in request payload")
				}

				filterMap := filter.(map[string]interface{})
				orFilters, ok := filterMap["or"].([]interface{})
				if !ok {
					t.Error("expected 'or' filter in request payload")
				}

				// Check that we have filters for all three "In progress" statuses
				expectedStatuses := map[string]bool{
					TrackStatusDemo:      false,
					TrackStatusRecording: false,
					TrackStatusMixing:    false,
				}

				for _, orFilter := range orFilters {
					filterEntry := orFilter.(map[string]interface{})
					if filterEntry["property"] != "Статус" {
						continue
					}
					statusField := filterEntry["status"].(map[string]interface{})
					statusName := statusField["equals"].(string)
					if _, exists := expectedStatuses[statusName]; exists {
						expectedStatuses[statusName] = true
					}
				}

				for status, found := range expectedStatuses {
					if !found {
						t.Errorf("expected filter for status '%s' not found", status)
					}
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
					bodyBytes, _ := io.ReadAll(req.Body) //nolint:errcheck
					validateReq := req.Clone(req.Context())
					validateReq.Body = io.NopCloser(bytes.NewReader(bodyBytes))
					tt.validateReq(t, validateReq)
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

func TestLoadAllTrackPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var payload map[string]interface{}
		err := json.NewDecoder(req.Body).Decode(&payload)
		if err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if _, ok := payload["filter"]; ok {
			t.Fatalf("did not expect filter for loading all tracks")
		}

		w.Header().Set("Content-Type", testContentTypeTweaks)
		err = json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"id": "bbbbbbbb-1234-1234-1234-bbbbbbbbbbbb",
					"properties": map[string]interface{}{
						"Название": map[string]interface{}{
							"title": []map[string]interface{}{
								{"plain_text": "Bravo"},
							},
						},
					},
				},
				{
					"id": "aaaaaaaa-1234-1234-1234-aaaaaaaaaaaa",
					"properties": map[string]interface{}{
						"Название": map[string]interface{}{
							"title": []map[string]interface{}{
								{"plain_text": "Alpha"},
							},
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	notion := NewNotion("test-token")
	notion.SetAPIBaseURL(server.URL + "/")

	tracks, err := notion.LoadAllTrackPages("db-tracks-all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(tracks))
	}

	if tracks[0].Title != "Alpha" || tracks[1].Title != "Bravo" {
		t.Fatalf("expected tracks to be sorted alphabetically, got %#v", tracks)
	}

	if tracks[0].Link != notionURL+"aaaaaaaa123412341234aaaaaaaaaaaa" {
		t.Fatalf("unexpected link: %s", tracks[0].Link)
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

func TestLoadReadyMixTweaksForTrack(t *testing.T) {
	const (
		dbID        = "mix-db-id"
		trackPageID = "track-page-id"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := path.Join("databases", dbID, "query")
		if r.Method != http.MethodPost || r.URL.Path != "/"+expectedPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		filter := payload["filter"].(map[string]interface{})
		andFilters := filter["and"].([]interface{})
		if len(andFilters) != 2 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		relationFilter := andFilters[0].(map[string]interface{})
		if relationFilter["property"] != "Песня" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		relation := relationFilter["relation"].(map[string]interface{})
		if relation["contains"] != trackPageID {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		statusFilter := andFilters[1].(map[string]interface{})
		status := statusFilter["status"].(map[string]interface{})
		if status["equals"] != TweakMixStatusReadyForWork {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"properties": map[string]interface{}{
						"Кратко": map[string]interface{}{
							"title": []map[string]interface{}{{"plain_text": "Fix vocal"}},
						},
						"Дорожка": map[string]interface{}{
							"rich_text": []map[string]interface{}{{"plain_text": "Lead"}},
						},
						"Начало интервала": map[string]interface{}{
							"rich_text": []map[string]interface{}{{"plain_text": "0:10"}},
						},
						"Конец интервала": map[string]interface{}{
							"rich_text": []map[string]interface{}{{"plain_text": "0:20"}},
						},
						"Пояснение": map[string]interface{}{
							"rich_text": []map[string]interface{}{{"plain_text": "Too loud"}},
						},
						"Автор (Manual)": map[string]interface{}{
							"people": []map[string]interface{}{{"name": "Kirill", "id": "user-id"}},
						},
					},
				},
			},
		}); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	n := NewNotion("test-token")
	n.SetAPIBaseURL(server.URL + "/")
	n.SetTweaksDBIDs("demo-db-id", dbID)

	tweaks, err := n.LoadReadyMixTweaksForTrack(trackPageID)
	if err != nil {
		t.Fatalf("LoadReadyMixTweaksForTrack returned error: %v", err)
	}
	want := []RenderTweak{
		{
			Summary:     "Fix vocal",
			TrackPart:   "Lead",
			Start:       "0:10",
			End:         "0:20",
			Explanation: "Too loud",
			Author:      "Kirill",
		},
	}
	if len(tweaks) != len(want) || tweaks[0] != want[0] {
		t.Fatalf("unexpected tweaks: got %#v, want %#v", tweaks, want)
	}
}

func TestCountUnreadyMixTweaksForTrack(t *testing.T) {
	const (
		dbID        = "mix-db-id"
		trackPageID = "track-page-id"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := path.Join("databases", dbID, "query")
		if r.Method != http.MethodPost || r.URL.Path != "/"+expectedPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		filter := payload["filter"].(map[string]interface{})
		andFilters := filter["and"].([]interface{})
		if len(andFilters) != 2 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		relation := andFilters[0].(map[string]interface{})["relation"].(map[string]interface{})
		if relation["contains"] != trackPageID {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		orFilters := andFilters[1].(map[string]interface{})["or"].([]interface{})
		if len(orFilters) != 2 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		expectedStatuses := map[string]bool{
			TweakMixStatusAnalysis: false,
			TweakMixStatusDeferred: false,
		}
		for _, rawFilter := range orFilters {
			status := rawFilter.(map[string]interface{})["status"].(map[string]interface{})
			statusName := status["equals"].(string)
			if _, ok := expectedStatuses[statusName]; !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			expectedStatuses[statusName] = true
		}
		for _, found := range expectedStatuses {
			if !found {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{
				{"id": "first-unready-tweak"},
				{"id": "second-unready-tweak"},
			},
		}); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	n := NewNotion("test-token")
	n.SetAPIBaseURL(server.URL + "/")
	n.SetTweaksDBIDs("demo-db-id", dbID)

	count, err := n.CountUnreadyMixTweaksForTrack(trackPageID)
	if err != nil {
		t.Fatalf("CountUnreadyMixTweaksForTrack returned error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 unready tweaks, got %d", count)
	}
}

func TestMoveReadyMixTweaksToWorkForTrack(t *testing.T) {
	const (
		dbID        = "mix-db-id"
		trackPageID = "track-page-id"
		firstTweak  = "first-tweak-id"
		secondTweak = "second-tweak-id"
	)

	patched := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + path.Join("databases", dbID, "query"):
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			filter := payload["filter"].(map[string]interface{})
			andFilters := filter["and"].([]interface{})
			relation := andFilters[0].(map[string]interface{})["relation"].(map[string]interface{})
			if relation["contains"] != trackPageID {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			status := andFilters[1].(map[string]interface{})["status"].(map[string]interface{})
			if status["equals"] != TweakMixStatusReadyForWork {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", testContentTypeTweaks)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{"id": firstTweak, "properties": map[string]interface{}{}},
					{"id": secondTweak, "properties": map[string]interface{}{}},
				},
			}); err != nil {
				t.Fatalf("failed to write response: %v", err)
			}
		case "/" + path.Join("pages", firstTweak), "/" + path.Join("pages", secondTweak):
			if r.Method != http.MethodPatch {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			props := payload["properties"].(map[string]interface{})
			statusProp := props["Статус"].(map[string]interface{})
			status := statusProp["status"].(map[string]interface{})
			patched[strings.TrimPrefix(r.URL.Path, "/pages/")] = status["name"].(string)

			w.Header().Set("Content-Type", testContentTypeTweaks)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{"id": "updated"}); err != nil {
				t.Fatalf("failed to write response: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	n := NewNotion("test-token")
	n.SetAPIBaseURL(server.URL + "/")
	n.SetTweaksDBIDs("demo-db-id", dbID)

	updated, err := n.MoveReadyMixTweaksToWorkForTrack(trackPageID)
	if err != nil {
		t.Fatalf("MoveReadyMixTweaksToWorkForTrack returned error: %v", err)
	}

	if updated != 2 {
		t.Fatalf("expected 2 updated tweaks, got %d", updated)
	}
	if patched[firstTweak] != TweakMixStatusInWork || patched[secondTweak] != TweakMixStatusInWork {
		t.Fatalf("unexpected patched statuses: %#v", patched)
	}
}
