package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type createPayload struct {
	Parent struct {
		DatabaseID string `json:"database_id"`
	} `json:"parent"`
	Properties map[string]interface{}   `json:"properties"`
	Children   []map[string]interface{} `json:"children,omitempty"`
}

type createResult struct {
	ID string `json:"id"`
}

func newCreatePayload(createRequest *CreateTaskRequest) *createPayload {
	payload := &createPayload{}
	payload.Parent.DatabaseID = createRequest.NotionDBID

	payload.Properties = map[string]interface{}{
		"Задача": map[string]interface{}{
			"title": []map[string]interface{}{
				{"text": map[string]string{"content": createRequest.TaskName}},
			},
		},
		"_timeWhenMovedToWork": map[string]interface{}{
			"date": map[string]string{
				"start": time.Now().Format(time.RFC3339),
			},
		},
		"Статус": map[string]interface{}{
			"select": map[string]string{
				"name": StatusNew,
			},
		},
	}

	if len(createRequest.Assignees) > 0 {
		assigneesPayload := make([]map[string]string, len(createRequest.Assignees))
		for i, id := range createRequest.Assignees {
			assigneesPayload[i] = map[string]string{
				"object": "user",
				"id":     id,
			}
		}

		payload.Properties["Исполнитель"] = map[string]interface{}{
			"people": assigneesPayload,
		}
	}

	if createRequest.Description != "" {
		children := []map[string]interface{}{
			{
				"object": "block",
				"type":   "paragraph",
				"paragraph": map[string]interface{}{
					"rich_text": []map[string]interface{}{
						{
							"type": "text",
							"text": map[string]string{
								"content": createRequest.Description,
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

type CreateTaskRequest struct {
	NotionDBID string

	TaskName    string
	Assignees   []string
	Description string

	Debug bool
}

func NewCreateTaskRequest() *CreateTaskRequest {
	return &CreateTaskRequest{}
}

func (n *Notion) CreateNotionTask(r *CreateTaskRequest) (string, error) {
	payload := newCreatePayload(r)

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("could not marshal request: %w", err)
	}

	if r.Debug {
		prettyPayload, _ := json.MarshalIndent(payload, "", "  ") //nolint:errcheck
		log.Println(string(prettyPayload))
		log.Println(notionAPI + "pages")
	}

	req, err := http.NewRequest("POST", notionAPI+"pages", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("could not create a request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+n.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := n.doWithRetries(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result createResult

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	cleanID := strings.ReplaceAll(result.ID, "-", "")
	url := notionURL + cleanID

	return url, nil
}
