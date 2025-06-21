package requestprocessor

import (
	"testing"
	"time"

	"github.com/gibsn/telegram_to_notion/internal/notion"

	"github.com/stretchr/testify/assert"
)

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
			name: "error: no /task",
			input: `test_task
@gibsn
test_description`,
			isPrivate: false,
			expectErr: true,
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
			command := extractCommand(tt.input)
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
		name          string
		repliedToText string
		input         string
		expectErr     bool
		want          *notion.SetDeadlineRequest
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := extractCommand(tt.input)
			command.repliedToText = tt.repliedToText

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
		name          string
		repliedToText string
		input         string
		expectErr     bool
		want          *notion.SetStatusRequest
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := extractCommand(tt.input)
			command.repliedToText = tt.repliedToText

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
