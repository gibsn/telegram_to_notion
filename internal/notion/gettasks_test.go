package notion

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

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
						People: []assignee{{Name: "Alice", ID: "uuid-alice"}},
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
				Assignees: []assignee{{Name: "Alice", ID: "uuid-alice"}},
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
						People: []assignee{
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
				Assignees: []assignee{
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
			errMessage: "missing assignee",
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
						People: []assignee{{Name: "Alice", ID: "uuid-alice"}},
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
				Assignees: []assignee{{Name: "Alice", ID: "uuid-alice"}},
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
						People: []assignee{{Name: "Alice", ID: "uuid-alice"}},
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
				Assignees: []assignee{{Name: "Alice", ID: "uuid-alice"}},
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
	tm, _ := time.ParseInLocation("2006-01-02", s, loc)

	return tm
}
