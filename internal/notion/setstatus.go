package notion

import (
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

	req, err := http.NewRequest("PATCH", n.apiBaseURL+"pages/"+pageID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := n.doWithRetries(req, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
