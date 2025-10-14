package requestprocessor

import (
	"testing"
	"time"

	"github.com/gibsn/telegram_to_notion/internal/notion"
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
