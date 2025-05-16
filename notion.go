package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type UserResolver struct {
	mapping map[string]string
}

func NewUserResolver() *UserResolver {
	r := &UserResolver{}

	r.mapping = map[string]string{
		"@alexander_zh": "9e8f4963-fd1c-4bb5-bdd2-7f29a9a8698a",
		"@vomadan":      "0724b18e-320d-4fce-87f6-95d69b51c2c0",
		"@fenyakolles":  "78694531-146f-4abd-b29b-093278cab708",
		"@nikitacmc":    "e6f7887a-7123-4a83-a5da-ded24467d5e2",
		"@Homesick94":   "3c02801c-1a5a-428f-b217-6d53032a21c9",
		"@bond_lullaby": "aea80e9c-7a69-4180-8a38-6d274af25f4c",
		"@gibsn":        "7439e2ca-75f8-4024-b170-620ef7ed08b1",
	}

	return r
}

func (r *UserResolver) Resolve(tgName string) string {
	return r.mapping[tgName]
}

type NotionPayload struct {
	Parent struct {
		DatabaseID string `json:"database_id"`
	} `json:"parent"`
	Properties map[string]interface{}   `json:"properties"`
	Children   []map[string]interface{} `json:"children,omitempty"`
}

func CreateNotionTask(token, dbID, taskName, assignee, description string) error {
	payload := NotionPayload{}
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

	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", "https://api.notion.com/v1/pages", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("Notion API error: %s", resp.Status)
	}

	return nil
}
