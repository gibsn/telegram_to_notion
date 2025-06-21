package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path"
	"strings"
	"time"
)

type loadPayload struct {
	Filter map[string]interface{} `json:"filter"`
}

type Assignee struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type loadResultProperty map[string]struct {
	Title []struct {
		PlainText string `json:"plain_text"`
	} `json:"title,omitempty"`
	People []Assignee `json:"People,omitempty"`
	Date   struct {
		Start string `json:"start"`
	} `json:"Date,omitempty"`
}

type loadResultEntry struct {
	ID         string             `json:"id"`
	Properties loadResultProperty `json:"properties"`
}

type loadResult struct {
	Results []loadResultEntry `json:"results"`
}

type Task struct {
	Title     string
	Assignees []Assignee
	Deadline  time.Time
	Link      string
}

func createTasksFilter() []map[string]interface{} {
	andFilters := make([]map[string]interface{}, 0, 3)
	for _, status := range []string{StatusBacklog, StatusDone, StatusArchived} {
		andFilters = append(andFilters, map[string]interface{}{
			"property": "Статус",
			"select": map[string]string{
				"does_not_equal": status,
			},
		})
	}
	andFilters = append(andFilters, map[string]interface{}{
		"property": "Статус",
		"select": map[string]bool{
			"is_not_empty": true,
		},
	})

	return andFilters
}

func parseTask(result loadResultEntry) (Task, error) {
	titleField, ok := result.Properties["Задача"]
	if !ok || len(titleField.Title) == 0 {
		return Task{}, fmt.Errorf("missing title")
	}

	taskName := titleField.Title[0].PlainText

	assigneeField, ok := result.Properties["Исполнитель"]
	if !ok || len(assigneeField.People) == 0 {
		return Task{}, fmt.Errorf("missing assignees")
	}

	assignees := assigneeField.People

	loc := time.Now().Location()

	var (
		deadline time.Time
		err      error
	)

	if dateField, ok := result.Properties["Дедлайн"]; ok && len(dateField.Date.Start) > 0 {
		deadline, err = time.ParseInLocation("2006-01-02", dateField.Date.Start, loc)
		if err != nil {
			log.Printf("Invalid deadline '%s': %v", deadline, err)
		}
	}

	taskURL := notionURL + strings.ReplaceAll(result.ID, "-", "")

	return Task{
		Title:     taskName,
		Assignees: assignees,
		Deadline:  deadline,
		Link:      taskURL,
	}, nil
}

func (n *Notion) LoadTasks(dbID string) ([]Task, error) {
	var payload loadPayload
	payload.Filter = map[string]interface{}{
		"and": createTasksFilter(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("could not marshal json payload: %w", err)
	}

	req, err := http.NewRequest(
		"POST", notionAPI+path.Join("databases", dbID, "query"),
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create a request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+n.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := n.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to Notion API failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("status code is %d", resp.StatusCode)
	}

	var result loadResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	tasks := make([]Task, 0, len(result.Results))

	for _, task := range result.Results {
		taskParsed, err := parseTask(task)
		if err != nil {
			log.Printf(
				"Could not load task (id: %s): invalid response fron Notion API: %v",
				task.ID, err,
			)
		}

		tasks = append(tasks, taskParsed)
	}

	return tasks, nil
}
