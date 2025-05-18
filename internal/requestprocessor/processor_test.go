package requestprocessor

import (
	"testing"

	"github.com/gibsn/telegram_to_notion/internal/notion"

	"github.com/stretchr/testify/assert"
)

func TestParseTelegramRequestMessage(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *notion.CreateTaskRequest
		expectErr bool
	}{
		{
			name: "один исполнитель, с описанием",
			input: `/task test_task
@gibsn
test_description`,
			want: &notion.CreateTaskRequest{
				TaskName:    "test_task",
				Assignees:   []string{"@gibsn"},
				Description: "test_description",
			},
		},
		{
			name: "два исполнителя через пробел",
			input: `/task test_task
@gibsn @alexander_zh
test_description`,
			want: &notion.CreateTaskRequest{
				TaskName:    "test_task",
				Assignees:   []string{"@gibsn", "@alexander_zh"},
				Description: "test_description",
			},
		},
		{
			name: "два исполнителя с несколькими пробелами",
			input: `/task test_task
@gibsn    @alexander_zh
test_description`,
			want: &notion.CreateTaskRequest{
				TaskName:    "test_task",
				Assignees:   []string{"@gibsn", "@alexander_zh"},
				Description: "test_description",
			},
		},
		{
			name: "ошибка: нет /task",
			input: `test_task
@gibsn
test_description`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTelegramRequestMessage(tt.input)

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
