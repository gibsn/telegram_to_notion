package requestprocessor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gibsn/telegram_to_notion/internal/notion"
	"github.com/gibsn/telegram_to_notion/internal/trackscache"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/stretchr/testify/assert"
)

func makeBotCommandEntities(text string) []tgbotapi.MessageEntity {
	// create a single bot_command entity for the first token that starts at offset 0
	end := len(text)
	for i, ch := range text {
		if ch == ' ' || ch == '\n' || ch == '\t' {
			end = i
			break
		}
	}
	if end == 0 || text[0] != '/' {
		return nil
	}
	return []tgbotapi.MessageEntity{{
		Type:   "bot_command",
		Offset: 0,
		Length: end,
	}}
}

func TestParseTaskCommand(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		isPrivate    bool
		fromUserName string
		want         *notion.CreateTaskRequest
		expectErr    bool
	}{
		// isPrivate = false
		{
			name: "one assignee, description present",
			input: `/task test_task
@gibsn
test_description`,
			isPrivate: false,
			want: &notion.CreateTaskRequest{
				TaskName:    "test_task",
				Assignees:   []string{"@gibsn"},
				Description: "test_description",
			},
		},
		{
			name: "one assignee, no description present",
			input: `/task test_task
@gibsn`,
			isPrivate: false,
			want: &notion.CreateTaskRequest{
				TaskName:    "test_task",
				Assignees:   []string{"@gibsn"},
				Description: "",
			},
		},
		{
			name: "multiple assignees, description present",
			input: `/task test_task
@gibsn @alexander_zh
test_description`,
			isPrivate: false,
			want: &notion.CreateTaskRequest{
				TaskName:    "test_task",
				Assignees:   []string{"@gibsn", "@alexander_zh"},
				Description: "test_description",
			},
		},
		{
			name: "multiple assignees, no description present",
			input: `/task test_task
@gibsn @alexander_zh`,
			isPrivate: false,
			want: &notion.CreateTaskRequest{
				TaskName:    "test_task",
				Assignees:   []string{"@gibsn", "@alexander_zh"},
				Description: "",
			},
		},
		{
			name: "multiple assignees separated with multiple spaces",
			input: `/task test_task
@gibsn      @alexander_zh`,
			isPrivate: false,
			want: &notion.CreateTaskRequest{
				TaskName:    "test_task",
				Assignees:   []string{"@gibsn", "@alexander_zh"},
				Description: "",
			},
		},
		{
			name:      "error: no task name",
			input:     `/task  `,
			expectErr: true,
		},
		{
			name: "error: no task name but description is present",
			input: `/task
test_description`,
			expectErr: true,
		},

		// isPrivate = true
		{
			name:         "private: only task name",
			input:        `/task test_task`,
			isPrivate:    true,
			fromUserName: "testuser",
			want: &notion.CreateTaskRequest{
				TaskName:    "test_task",
				Assignees:   []string{"@testuser"},
				Description: "",
			},
		},
		{
			name: "private: only task name and description",
			input: `/task test_task
test_description`,
			isPrivate:    true,
			fromUserName: "testuser",
			want: &notion.CreateTaskRequest{
				TaskName:    "test_task",
				Assignees:   []string{"@testuser"},
				Description: "test_description",
			},
		},
		{
			name: "private: assignee is present by mistake, so it goes to description",
			input: `/task test_task
@gibsn
test_description`,
			isPrivate:    true,
			fromUserName: "testuser",
			want: &notion.CreateTaskRequest{
				TaskName:    "test_task",
				Assignees:   []string{"@testuser"},
				Description: "@gibsn\ntest_description",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := extractCommand(tt.input, makeBotCommandEntities(tt.input))
			assert.NoError(t, err)
			command := cmd
			command.isPrivate = tt.isPrivate
			command.fromUserName = tt.fromUserName

			got, err := parseTaskCommand(command)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.TaskName, got.TaskName)
				assert.Equal(t, tt.want.Assignees, got.Assignees)
				assert.Equal(t, tt.want.Description, got.Description)
			}
		})
	}
}

func TestParseSetDeadlineCommand(t *testing.T) {
	tests := []struct {
		name              string
		repliedToText     string
		repliedToEntities []tgbotapi.MessageEntity
		input             string
		expectErr         bool
		want              *notion.SetDeadlineRequest
	}{
		{
			name:          "valid deadline",
			repliedToText: "Task created: https://www.notion.so/abc123",
			input:         "/deadline 2024-12-31",
			expectErr:     false,
			want: &notion.SetDeadlineRequest{
				TaskLink: "https://www.notion.so/abc123",
				Deadline: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:          "no replied text",
			repliedToText: "",
			input:         "/deadline 2024-12-31",
			expectErr:     true,
			want:          nil,
		},
		{
			name:          "no task link in replied text",
			repliedToText: "This is just a regular message",
			input:         "/deadline 2024-12-31",
			expectErr:     true,
			want:          nil,
		},
		{
			name:          "invalid date format",
			repliedToText: "Task created: https://www.notion.so/abc123",
			input:         "/deadline 2024/12/31",
			expectErr:     true,
			want:          nil,
		},
		{
			name:          "empty date",
			repliedToText: "Task created: https://www.notion.so/abc123",
			input:         "/deadline",
			expectErr:     true,
			want:          nil,
		},
		{
			name:          "multiple task links - uses first one",
			repliedToText: "https://www.notion.so/abc123 and https://www.notion.so/def456",
			input:         "/deadline 2024-12-31",
			expectErr:     false,
			want: &notion.SetDeadlineRequest{
				TaskLink: "https://www.notion.so/abc123",
				Deadline: time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:          "task link with different format",
			repliedToText: "Check this out: https://www.notion.so/task-123-456",
			input:         "/deadline 2024-01-15",
			expectErr:     false,
			want: &notion.SetDeadlineRequest{
				TaskLink: "https://www.notion.so/task-123-456",
				Deadline: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:          "reply to hyperlink",
			repliedToText: "My awesome task",
			repliedToEntities: []tgbotapi.MessageEntity{
				{
					Type: "text_link",
					URL:  "https://www.notion.so/hyperlink-task",
				},
			},
			input:     "/deadline 2025-01-01",
			expectErr: false,
			want: &notion.SetDeadlineRequest{
				TaskLink: "https://www.notion.so/hyperlink-task",
				Deadline: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := extractCommand(tt.input, makeBotCommandEntities(tt.input))
			assert.NoError(t, err)
			command := cmd
			command.repliedToText = tt.repliedToText
			command.repliedToEntities = tt.repliedToEntities

			processor := NewRequestProcessor(nil, "", nil)
			got, err := processor.parseSetDeadlineCommand(command)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.TaskLink, got.TaskLink)
				assert.Equal(t, tt.want.Deadline, got.Deadline)
			}
		})
	}
}

func TestParseDoneCommand(t *testing.T) {
	tests := []struct {
		name              string
		repliedToText     string
		repliedToEntities []tgbotapi.MessageEntity
		input             string
		expectErr         bool
		want              *notion.SetStatusRequest
	}{
		{
			name:          "valid done command",
			repliedToText: "Task created: https://www.notion.so/abc123",
			input:         "/done",
			expectErr:     false,
			want: &notion.SetStatusRequest{
				TaskLink: "https://www.notion.so/abc123",
				Status:   notion.StatusDone,
			},
		},
		{
			name:          "no replied text",
			repliedToText: "",
			input:         "/done",
			expectErr:     true,
			want:          nil,
		},
		{
			name:          "no task link in replied text",
			repliedToText: "This is just a regular message",
			input:         "/done",
			expectErr:     true,
			want:          nil,
		},
		{
			name:          "multiple task links - uses first one",
			repliedToText: "https://www.notion.so/abc123 and https://www.notion.so/def456",
			input:         "/done",
			expectErr:     false,
			want: &notion.SetStatusRequest{
				TaskLink: "https://www.notion.so/abc123",
				Status:   notion.StatusDone,
			},
		},
		{
			name:          "task link with different format",
			repliedToText: "Check this out: https://www.notion.so/task-123-456",
			input:         "/done",
			expectErr:     false,
			want: &notion.SetStatusRequest{
				TaskLink: "https://www.notion.so/task-123-456",
				Status:   notion.StatusDone,
			},
		},
		{
			name:          "done command with extra text (should be ignored)",
			repliedToText: "Task created: https://www.notion.so/abc123",
			input:         "/done some extra text",
			expectErr:     false,
			want: &notion.SetStatusRequest{
				TaskLink: "https://www.notion.so/abc123",
				Status:   notion.StatusDone,
			},
		},
		{
			name:          "reply to hyperlink",
			repliedToText: "My awesome task",
			repliedToEntities: []tgbotapi.MessageEntity{
				{
					Type: "text_link",
					URL:  "https://www.notion.so/hyperlink-task-done",
				},
			},
			input:     "/done",
			expectErr: false,
			want: &notion.SetStatusRequest{
				TaskLink: "https://www.notion.so/hyperlink-task-done",
				Status:   notion.StatusDone,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := extractCommand(tt.input, makeBotCommandEntities(tt.input))
			assert.NoError(t, err)
			command := cmd
			command.repliedToText = tt.repliedToText
			command.repliedToEntities = tt.repliedToEntities

			processor := NewRequestProcessor(nil, "", nil)
			got, err := processor.parseDoneCommand(command)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.TaskLink, got.TaskLink)
				assert.Equal(t, tt.want.Status, got.Status)
			}
		})
	}
}

func TestParseTasksCommand(t *testing.T) {
	tests := []struct {
		name         string
		fromUserName string
		expectErr    bool
		want         string
	}{
		{
			name:         "valid user",
			fromUserName: "gibsn",
			expectErr:    false,
			want:         "7439e2ca-75f8-4024-b170-620ef7ed08b1",
		},
		{
			name:         "another valid user",
			fromUserName: "alexander_zh",
			expectErr:    false,
			want:         "9e8f4963-fd1c-4bb5-bdd2-7f29a9a8698a",
		},
		{
			name:         "unknown user",
			fromUserName: "unknown_user",
			expectErr:    true,
			want:         "",
		},
		{
			name:         "empty username",
			fromUserName: "",
			expectErr:    true,
			want:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := commandCommon{
				fromUserName: tt.fromUserName,
				chatID:       -123456789, // Mock chat ID
			}

			processor := NewRequestProcessor(nil, "", nil)
			got, err := processor.parseTasksCommand(command)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseTracksCommand(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantAll   bool
		expectErr bool
	}{
		{
			name:    "without arguments loads only in progress",
			input:   "/tracks",
			wantAll: false,
		},
		{
			name:    "all argument loads all tracks",
			input:   "/tracks all",
			wantAll: true,
		},
		{
			name:    "all argument is case insensitive",
			input:   "/tracks ALL",
			wantAll: true,
		},
		{
			name:      "unknown argument returns error",
			input:     "/tracks archived",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := extractCommand(tt.input, makeBotCommandEntities(tt.input))
			assert.NoError(t, err)

			got, err := parseTracksCommand(cmd)

			if tt.expectErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantAll, got)
		})
	}
}

func TestProcessTracks(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantPrefix string
		wantFilter bool
	}{
		{
			name:       "default returns in progress tracks",
			input:      "/tracks",
			wantPrefix: "Tracks in progress:\n\n",
			wantFilter: true,
		},
		{
			name:       "all returns all tracks",
			input:      "/tracks all",
			wantPrefix: "All tracks:\n\n",
			wantFilter: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				var payload map[string]interface{}
				err := json.NewDecoder(req.Body).Decode(&payload)
				assert.NoError(t, err)

				filter, hasFilter := payload["filter"]
				assert.Equal(t, tt.wantFilter, hasFilter)
				if tt.wantFilter {
					filterMap, ok := filter.(map[string]interface{})
					assert.True(t, ok)
					_, ok = filterMap["or"]
					assert.True(t, ok)
				}

				w.Header().Set("Content-Type", "application/json")
				err = json.NewEncoder(w).Encode(map[string]interface{}{
					"results": []map[string]interface{}{
						{
							"id": "12345678-1234-1234-1234-123456789abc",
							"properties": map[string]interface{}{
								"Название": map[string]interface{}{
									"title": []map[string]interface{}{
										{"plain_text": "Beta & Co"},
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
				assert.NoError(t, err)
			}))
			defer server.Close()

			n := notion.NewNotion("test-token")
			n.SetAPIBaseURL(server.URL + "/")
			p := NewRequestProcessor(n, "", nil)
			p.SetTracksDBID("tracks-db-id")

			cmd, err := extractCommand(tt.input, makeBotCommandEntities(tt.input))
			assert.NoError(t, err)

			reply, err := p.processTracks(cmd)
			assert.NoError(t, err)
			assert.True(t, strings.HasPrefix(reply, tt.wantPrefix))
			expectedAlpha := "1. <a href=\"https://www.notion.so/" +
				"aaaaaaaa123412341234aaaaaaaaaaaa\">Alpha</a>"
			expectedBeta := "2. <a href=\"https://www.notion.so/" +
				"12345678123412341234123456789abc\">Beta &amp; Co</a>"
			assert.Contains(t, reply, expectedAlpha)
			assert.Contains(t, reply, expectedBeta)
		})
	}
}

func TestParseTweakCommand(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *TweakRequest
		expectErr bool
	}{
		{
			name:  "demo minimal",
			input: "/tweak demo Track1\nEditA",
			want: &TweakRequest{
				Mode:      tweakModeDemo,
				TrackName: "Track1",
				EditName:  "EditA",
			},
		},
		{
			name:  "demo multiword",
			input: "/tweak demo Track One\nEdit Two",
			want: &TweakRequest{
				Mode:      tweakModeDemo,
				TrackName: "Track One",
				EditName:  "Edit Two",
			},
		},
		{
			name:  "demo with start time on third line",
			input: "/tweak demo Track1\nEditA\n1:23",
			want: &TweakRequest{
				Mode:      tweakModeDemo,
				TrackName: "Track1",
				EditName:  "EditA",
				Start:     "1:23",
			},
		},
		{
			name:  "demo with start and end time on third line",
			input: "/tweak demo Track1\nEditA\n1:23 2:34",
			want: &TweakRequest{
				Mode:      tweakModeDemo,
				TrackName: "Track1",
				EditName:  "EditA",
				Start:     "1:23",
				End:       "2:34",
			},
		},
		{
			name:  "demo with start, end and description",
			input: "/tweak demo Track1\nEditA\n1:23 2:34\nSome explanation",
			want: &TweakRequest{
				Mode:        tweakModeDemo,
				TrackName:   "Track1",
				EditName:    "EditA",
				Start:       "1:23",
				End:         "2:34",
				Description: "Some explanation",
			},
		},
		{
			name:  "demo with description on second line",
			input: "/tweak demo Track1\nEditA\nSome explanation",
			want: &TweakRequest{
				Mode:        tweakModeDemo,
				TrackName:   "Track1",
				EditName:    "EditA",
				Description: "Some explanation",
			},
		},
		{
			name:  "invalid time format treated as description",
			input: "/tweak demo Track1\nEditA\n1:2",
			want: &TweakRequest{
				Mode:        tweakModeDemo,
				TrackName:   "Track1",
				EditName:    "EditA",
				Description: "1:2",
			},
		},
		{
			name:  "demo with multi-word edit name",
			input: "/tweak demo Track1\nMy Awesome Edit",
			want: &TweakRequest{
				Mode:      tweakModeDemo,
				TrackName: "Track1",
				EditName:  "My Awesome Edit",
			},
		},
		{
			name:  "demo with multi-word edit name and start time",
			input: "/tweak demo Track1\nMy Awesome Edit\n1:23",
			want: &TweakRequest{
				Mode:      tweakModeDemo,
				TrackName: "Track1",
				EditName:  "My Awesome Edit",
				Start:     "1:23",
			},
		},
		{
			name:  "demo with multi-word edit name, start and end",
			input: "/tweak demo Track1\nMy Awesome Edit\n1:23 2:34",
			want: &TweakRequest{
				Mode:      tweakModeDemo,
				TrackName: "Track1",
				EditName:  "My Awesome Edit",
				Start:     "1:23",
				End:       "2:34",
			},
		},
		{
			name:  "demo with multi-word edit name and description",
			input: "/tweak demo Track1\nMy Awesome Edit\nSome explanation",
			want: &TweakRequest{
				Mode:        tweakModeDemo,
				TrackName:   "Track1",
				EditName:    "My Awesome Edit",
				Description: "Some explanation",
			},
		},
		{
			name:  "demo with multi-word edit name, times and description",
			input: "/tweak demo Track1\nMy Awesome Edit\n1:23 2:34\nSome explanation",
			want: &TweakRequest{
				Mode:        tweakModeDemo,
				TrackName:   "Track1",
				EditName:    "My Awesome Edit",
				Start:       "1:23",
				End:         "2:34",
				Description: "Some explanation",
			},
		},
		{
			name:      "unknown mode",
			input:     "/tweak something Track1 EditA",
			expectErr: true,
		},
		{
			name:      "too few args",
			input:     "/tweak demo Track1",
			expectErr: true,
		},
		{
			name:      "empty body",
			input:     "/tweak",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := extractCommand(tt.input, makeBotCommandEntities(tt.input))
			assert.NoError(t, err)
			processor := NewRequestProcessor(nil, "", nil)
			got, err := processor.parseTweakCommand(cmd)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want.Mode, got.Mode)
				assert.Equal(t, tt.want.TrackName, got.TrackName)
				assert.Equal(t, tt.want.EditName, got.EditName)
				assert.Equal(t, tt.want.Start, got.Start)
				assert.Equal(t, tt.want.End, got.End)
				assert.Equal(t, tt.want.Description, got.Description)
			}
		})
	}
}

func TestParseTweakRenderCommand(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *TweakRenderRequest
		expectErr bool
	}{
		{
			name:  "single word track",
			input: "/tweak render Track1 2",
			want:  &TweakRenderRequest{TrackName: "Track1", Iteration: 2},
		},
		{
			name:  "multi word track",
			input: "/tweak render Track One 12",
			want:  &TweakRenderRequest{TrackName: "Track One", Iteration: 12},
		},
		{
			name:      "iteration is not a number",
			input:     "/tweak render Track One x",
			expectErr: true,
		},
		{
			name:      "iteration is zero",
			input:     "/tweak render Track One 0",
			expectErr: true,
		},
		{
			name:      "missing track",
			input:     "/tweak render 2",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := extractCommand(tt.input, makeBotCommandEntities(tt.input))
			assert.NoError(t, err)

			got, err := parseTweakRenderCommand(cmd)

			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseTweakToWorkCommand(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *TweakToWorkRequest
		expectErr bool
	}{
		{
			name:  "single word track",
			input: "/tweak towork Track1",
			want:  &TweakToWorkRequest{TrackName: "Track1"},
		},
		{
			name:  "multi word track",
			input: "/tweak towork Track One",
			want:  &TweakToWorkRequest{TrackName: "Track One"},
		},
		{
			name:      "missing track",
			input:     "/tweak towork",
			expectErr: true,
		},
		{
			name:      "wrong subcommand",
			input:     "/tweak render Track One 1",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := extractCommand(tt.input, makeBotCommandEntities(tt.input))
			assert.NoError(t, err)

			got, err := parseTweakToWorkCommand(cmd)

			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProcessTweakRender(t *testing.T) {
	const (
		tracksDBID = "tracks-db-id"
		tweaksDBID = "tweaks-db-id"
		trackID    = "track-page-id"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/databases/" + tracksDBID + "/query":
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id": trackID,
						"properties": map[string]interface{}{
							"Название": map[string]interface{}{
								"title": []map[string]interface{}{{"plain_text": "Track One"}},
							},
						},
					},
				},
			})
			assert.NoError(t, err)
		case "/v1/databases/" + tweaksDBID + "/query":
			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			assert.NoError(t, err)
			filter := payload["filter"].(map[string]interface{})
			andFilters := filter["and"].([]interface{})
			relation := andFilters[0].(map[string]interface{})["relation"].(map[string]interface{})
			assert.Equal(t, trackID, relation["contains"])
			statusFilter := andFilters[1].(map[string]interface{})["status"].(map[string]interface{})

			if statusFilter["equals"] == notion.TweakMixStatusAnalysis {
				w.Header().Set("Content-Type", "application/json")
				err = json.NewEncoder(w).Encode(map[string]interface{}{
					"results": []map[string]interface{}{
						{"id": "unready-1"},
						{"id": "unready-2"},
					},
				})
				assert.NoError(t, err)
				return
			}
			assert.Equal(t, notion.TweakMixStatusReadyForWork, statusFilter["equals"])

			w.Header().Set("Content-Type", "application/json")
			err = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"properties": map[string]interface{}{
							"Кратко": map[string]interface{}{
								"title": []map[string]interface{}{{"plain_text": "Fix vocal"}},
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
								"people": []map[string]interface{}{{"name": "Kirill"}},
							},
						},
					},
				},
			})
			assert.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	n := notion.NewNotion("test-token")
	n.SetAPIBaseURL(server.URL + "/v1/")
	n.SetTweaksDBIDs("demo-db-id", tweaksDBID)
	tracksCache := trackscache.NewTracksCache(n, tracksDBID, time.Minute)
	assert.NoError(t, tracksCache.RefreshCache())

	p := NewRequestProcessor(n, "", nil)
	p.SetTracksCache(tracksCache)
	input := "/tweak render Track One 3"
	cmd, err := extractCommand(input, makeBotCommandEntities(input))
	assert.NoError(t, err)

	reply, doc, err := p.processTweakRender(cmd)

	assert.NoError(t, err)
	assert.Equal(
		t,
		"Generated 1 tweak for "+
			"<a href=\"https://www.notion.so/trackpageid\">Track One</a>\n"+
			"Unready tweaks left: 2",
		reply,
	)
	assert.NotNil(t, doc)
	assert.Equal(t, "Правки Track One 3.pdf", doc.FileName)
	assert.True(t, strings.HasPrefix(string(doc.Bytes), "%PDF-"))
}

func TestTweakRenderCaption(t *testing.T) {
	tests := []struct {
		name       string
		count      int
		trackName  string
		trackID    string
		wantResult string
	}{
		{
			name:      "one tweak",
			count:     1,
			trackName: "Track <One>",
			trackID:   "aaaaaaaa-1234-1234-1234-aaaaaaaaaaaa",
			wantResult: "Generated 1 tweak for " +
				"<a href=\"https://www.notion.so/aaaaaaaa123412341234aaaaaaaaaaaa\">" +
				"Track &lt;One&gt;</a>\nUnready tweaks left: 3",
		},
		{
			name:      "two tweaks",
			count:     2,
			trackName: "Track Two",
			trackID:   "track-2",
			wantResult: "Generated 2 tweaks for " +
				"<a href=\"https://www.notion.so/track2\">Track Two</a>\nUnready tweaks left: 3",
		},
		{
			name:      "five tweaks",
			count:     5,
			trackName: "Track Five",
			trackID:   "track-5",
			wantResult: "Generated 5 tweaks for " +
				"<a href=\"https://www.notion.so/track5\">Track Five</a>\nUnready tweaks left: 3",
		},
		{
			name:      "eleven tweaks",
			count:     11,
			trackName: "Track Eleven",
			trackID:   "track-11",
			wantResult: "Generated 11 tweaks for " +
				"<a href=\"https://www.notion.so/track11\">Track Eleven</a>\nUnready tweaks left: 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tweakRenderCaption(tt.trackName, tt.trackID, tt.count, 3)

			assert.Equal(t, tt.wantResult, got)
		})
	}
}

func TestProcessTweakRenderNoReadyTweaks(t *testing.T) {
	const (
		tracksDBID = "tracks-db-id"
		tweaksDBID = "tweaks-db-id"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/databases/" + tracksDBID + "/query":
			err := json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id": "track-page-id",
						"properties": map[string]interface{}{
							"Название": map[string]interface{}{
								"title": []map[string]interface{}{{"plain_text": "Track One"}},
							},
						},
					},
				},
			})
			assert.NoError(t, err)
		case "/v1/databases/" + tweaksDBID + "/query":
			err := json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{},
			})
			assert.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	n := notion.NewNotion("test-token")
	n.SetAPIBaseURL(server.URL + "/v1/")
	n.SetTweaksDBIDs("demo-db-id", tweaksDBID)
	tracksCache := trackscache.NewTracksCache(n, tracksDBID, time.Minute)
	assert.NoError(t, tracksCache.RefreshCache())

	p := NewRequestProcessor(n, "", nil)
	p.SetTracksCache(tracksCache)
	input := "/tweak render Track One 3"
	cmd, err := extractCommand(input, makeBotCommandEntities(input))
	assert.NoError(t, err)

	reply, doc, err := p.processTweakRender(cmd)

	assert.NoError(t, err)
	assert.Nil(t, doc)
	assert.Equal(t, "No tweaks found for track \"Track One\"", reply)
}

func TestProcessTweakToWork(t *testing.T) {
	const (
		tracksDBID = "tracks-db-id"
		tweaksDBID = "tweaks-db-id"
		trackID    = "track-page-id"
		tweakID    = "tweak-page-id"
	)

	var patchedStatuses []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/databases/" + tracksDBID + "/query":
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id": trackID,
						"properties": map[string]interface{}{
							"Название": map[string]interface{}{
								"title": []map[string]interface{}{{"plain_text": "Track One"}},
							},
						},
					},
				},
			})
			assert.NoError(t, err)
		case "/v1/databases/" + tweaksDBID + "/query":
			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			assert.NoError(t, err)
			filter := payload["filter"].(map[string]interface{})
			andFilters := filter["and"].([]interface{})
			relation := andFilters[0].(map[string]interface{})["relation"].(map[string]interface{})
			assert.Equal(t, trackID, relation["contains"])

			w.Header().Set("Content-Type", "application/json")
			err = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id":         tweakID,
						"properties": map[string]interface{}{},
					},
				},
			})
			assert.NoError(t, err)
		case "/v1/pages/" + tweakID:
			assert.Equal(t, http.MethodPatch, r.Method)
			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			assert.NoError(t, err)
			props := payload["properties"].(map[string]interface{})
			statusProp := props["Статус"].(map[string]interface{})
			status := statusProp["status"].(map[string]interface{})
			patchedStatuses = append(patchedStatuses, status["name"].(string))
			w.Header().Set("Content-Type", "application/json")
			err = json.NewEncoder(w).Encode(map[string]interface{}{"id": tweakID})
			assert.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	n := notion.NewNotion("test-token")
	n.SetAPIBaseURL(server.URL + "/v1/")
	n.SetTweaksDBIDs("demo-db-id", tweaksDBID)
	tracksCache := trackscache.NewTracksCache(n, tracksDBID, time.Minute)
	assert.NoError(t, tracksCache.RefreshCache())

	p := NewRequestProcessor(n, "", nil)
	p.SetTracksCache(tracksCache)
	input := "/tweak towork Track One"
	cmd, err := extractCommand(input, makeBotCommandEntities(input))
	assert.NoError(t, err)

	reply, err := p.processTweakToWork(cmd)

	assert.NoError(t, err)
	assert.Equal(
		t,
		"Moved 1 tweak for <a href=\"https://www.notion.so/trackpageid\">Track One</a> to work",
		reply,
	)
	assert.Equal(t, []string{notion.TweakMixStatusInWork}, patchedStatuses)
}

func TestProcessTweakToWorkNoReadyTweaks(t *testing.T) {
	const (
		tracksDBID = "tracks-db-id"
		tweaksDBID = "tweaks-db-id"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/databases/" + tracksDBID + "/query":
			err := json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id": "track-page-id",
						"properties": map[string]interface{}{
							"Название": map[string]interface{}{
								"title": []map[string]interface{}{{"plain_text": "Track One"}},
							},
						},
					},
				},
			})
			assert.NoError(t, err)
		case "/v1/databases/" + tweaksDBID + "/query":
			err := json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{},
			})
			assert.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	n := notion.NewNotion("test-token")
	n.SetAPIBaseURL(server.URL + "/v1/")
	n.SetTweaksDBIDs("demo-db-id", tweaksDBID)
	tracksCache := trackscache.NewTracksCache(n, tracksDBID, time.Minute)
	assert.NoError(t, tracksCache.RefreshCache())

	p := NewRequestProcessor(n, "", nil)
	p.SetTracksCache(tracksCache)
	input := "/tweak towork Track One"
	cmd, err := extractCommand(input, makeBotCommandEntities(input))
	assert.NoError(t, err)

	reply, err := p.processTweakToWork(cmd)

	assert.NoError(t, err)
	assert.Equal(t, "No ready tweaks found for track \"Track One\"", reply)
}

func TestCreateMessageLink(t *testing.T) {
	processor := NewRequestProcessor(nil, "", nil)

	tests := []struct {
		name      string
		chatID    int64
		messageID int
		isPrivate bool
		expected  string
	}{
		{
			name:      "group chat link",
			chatID:    -1001234567890,
			messageID: 123,
			isPrivate: false,
			expected:  "https://t.me/c/1234567890/123",
		},
		{
			name:      "private chat link",
			chatID:    123456789,
			messageID: 456,
			isPrivate: true,
			expected:  "",
		},
		{
			name:      "zero message ID",
			chatID:    -1001234567890,
			messageID: 0,
			isPrivate: false,
			expected:  "",
		},
		{
			name:      "regular group chat link (without 100 prefix)",
			chatID:    -4910620546,
			messageID: 274,
			isPrivate: false,
			expected:  "https://t.me/c/4910620546/274",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.createMessageLink(tt.chatID, tt.messageID, tt.isPrivate)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractCommand_TaskWithAndWithoutMention(t *testing.T) {
	cases := []struct {
		name      string
		text      string
		rest      string
		expectErr bool
	}{
		{"without_mention", "/task do something", "do something", false},
		{"with_mention", "/task@bot_name do something", "do something", false},
		{"error: no /task", "test_task\n@gibsn", "do something", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd, err := extractCommand(c.text, makeBotCommandEntities(c.text))
			if c.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "/task", cmd.command)
				assert.Equal(t, c.rest, cmd.restOfMessage)
			}
		})
	}
}

func TestParseAgendaCommand(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *notion.CreateTaskRequest
		expectErr bool
	}{
		{
			name:  "single word task name",
			input: "/agenda Sync",
			want: &notion.CreateTaskRequest{
				TaskName:  "Sync",
				Assignees: nil,
			},
		},
		{
			name:  "multi word task name",
			input: "/agenda Weekly team sync",
			want: &notion.CreateTaskRequest{
				TaskName:  "Weekly team sync",
				Assignees: nil,
			},
		},
		{
			name:  "task name with leading and trailing spaces",
			input: "/agenda   Plan sprint   ",
			want: &notion.CreateTaskRequest{
				TaskName:  "Plan sprint",
				Assignees: nil,
			},
		},
		{
			name:      "error: empty task name",
			input:     "/agenda",
			expectErr: true,
		},
		{
			name:      "error: only spaces as task name",
			input:     "/agenda   ",
			expectErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := extractCommand(tt.input, makeBotCommandEntities(tt.input))
			assert.NoError(t, err)
			command := cmd

			got, err := parseAgendaCommand(command)

			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want.TaskName, got.TaskName)
			assert.Empty(t, got.Assignees)
		})
	}
}

func TestAllNotionUserIDs(t *testing.T) {
	r := NewUserResolver()
	ids := r.AllNotionUserIDs()
	// UserResolver has 7 known users (see userresolver.go)
	assert.Len(t, ids, 7)
	// All IDs should be non-empty UUIDs
	for _, id := range ids {
		assert.NotEmpty(t, id)
		assert.Contains(t, id, "-")
	}
}

func TestProcessAgenda(t *testing.T) {
	const testDBID = "test-db-id"
	const testPageID = "12345678-1234-1234-1234-123456789abc"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/pages" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var payload struct {
			Properties map[string]interface{} `json:"properties"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		taskField, _ := payload.Properties["Задача"].(map[string]interface{})
		titleArr, _ := taskField["title"].([]interface{})
		if len(titleArr) > 0 {
			titleObj, _ := titleArr[0].(map[string]interface{})
			textObj, _ := titleObj["text"].(map[string]interface{})
			if content, _ := textObj["content"].(string); content != "Agenda: Weekly sync" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		assigneeField, _ := payload.Properties["Исполнитель"].(map[string]interface{})
		people, _ := assigneeField["people"].([]interface{})
		expectedCount := len(NewUserResolver().AllNotionUserIDs())
		if len(people) != expectedCount {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": testPageID}) //nolint:errcheck
	}))
	defer server.Close()

	n := notion.NewNotion("test-token")
	n.SetAPIBaseURL(server.URL + "/v1/")
	p := NewRequestProcessor(n, testDBID, nil)

	cmd, err := extractCommand("/agenda Weekly sync", makeBotCommandEntities("/agenda Weekly sync"))
	assert.NoError(t, err)
	message := cmd

	reply, err := p.processAgenda(message)
	assert.NoError(t, err)
	assert.Contains(t, reply, "Agenda created:")
	assert.Contains(t, reply, "https://www.notion.so/")
	assert.Contains(t, reply, "12345678123412341234123456789abc")
}
