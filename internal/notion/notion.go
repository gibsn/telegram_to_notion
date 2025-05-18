package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type notionPayload struct {
	Parent struct {
		DatabaseID string `json:"database_id"`
	} `json:"parent"`
	Properties map[string]interface{}   `json:"properties"`
	Children   []map[string]interface{} `json:"children,omitempty"`
}

type notionResult struct {
	ID string `json:"id"`
}

func newNotionPayload(dbID, taskName, assignee, description string) *notionPayload {
	payload := &notionPayload{}
	payload.Parent.DatabaseID = dbID

	payload.Properties = map[string]interface{}{
		"Задача": map[string]interface{}{
			"title": []map[string]interface{}{
				{"text": map[string]string{"content": taskName}},
			},
		},
		"Исполнитель": map[string]interface{}{
			"people": []map[string]string{
				{"object": "user", "id": assignee},
			},
		},
		"_timeWhenMovedToWork": map[string]interface{}{
			"date": map[string]string{
				"start": time.Now().Format(time.RFC3339),
			},
		},
		"Статус": map[string]interface{}{
			"select": map[string]string{
				"name": "новая",
			},
		},
	}
	if description != "" {
		children := []map[string]interface{}{
			{
				"object": "block",
				"type":   "paragraph",
				"paragraph": map[string]interface{}{
					"rich_text": []map[string]interface{}{
						{
							"type": "text",
							"text": map[string]string{
								"content": description,
							},
						},
					},
				},
			},
		}
		payload.Children = children
	}

	return payload
}

func CreateNotionTask(token, dbID, taskName, assignee, description string) (string, error) {
	payload := newNotionPayload(dbID, taskName, assignee, description)

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("could not marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.notion.com/v1/pages", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("could not create a request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("notion API error: %s", resp.Status)
	}

	var result notionResult

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	cleanID := strings.ReplaceAll(result.ID, "-", "")
	url := fmt.Sprintf("https://www.notion.so/%s", cleanID)

	return url, nil
}
