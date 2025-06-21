package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type SetStatusRequest struct {
	TaskLink string
	Status   string

	Debug bool
}

func (n *Notion) SetStatus(setRequest *SetStatusRequest) error {
	pageID := extractPageID(setRequest.TaskLink)
	if pageID == "" {
		return fmt.Errorf("invalid task link %s", setRequest.TaskLink)
	}

	payload := map[string]interface{}{
		"properties": map[string]interface{}{
			"Статус": map[string]interface{}{
				"select": map[string]string{
					"name": setRequest.Status,
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("PATCH", notionAPI+"pages/"+pageID, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+n.token)
	req.Header.Set("Notion-Version", "2022-06-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.doWithRetries(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
