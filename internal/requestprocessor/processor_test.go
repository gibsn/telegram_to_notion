package requestprocessor

import (
	"testing"

	"github.com/gibsn/telegram_to_notion/internal/notion"

	"github.com/stretchr/testify/assert"
)

func TestParseTelegramRequestMessage(t *testing.T) {
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
