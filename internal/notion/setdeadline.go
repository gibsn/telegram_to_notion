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

type SetDeadlineRequest struct {
	NotionDBID string

	TaskLink string
	Deadline time.Time

	Debug bool
}

func (n *Notion) SetDeadline(setRequest *SetDeadlineRequest) error {
	pageID := extractPageID(setRequest.TaskLink)
	if pageID == "" {
		return fmt.Errorf("invalid notion link")
	}

	isoDate := setRequest.Deadline.Format(time.RFC3339)

	payload := map[string]interface{}{
		"properties": map[string]interface{}{
			"Deadline": map[string]interface{}{
				"date": map[string]string{
					"start": isoDate,
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

	var resp *http.Response

	for i := 1; i <= retriesCreate; i++ {
		resp, err = n.client.Do(req)

		if err != nil || resp.StatusCode >= 300 {
			log.Printf("request to Notion API failed: %s", err)
			if resp != nil {
				resp.Body.Close()
			}

			if i < retriesCreate {
				log.Printf("retrying request to Notion API")
				continue
			}

			if err == nil {
				err = fmt.Errorf("status code is %d", resp.StatusCode)
			}

			return fmt.Errorf("request to Notion API failed: %w", err)
		}

		defer resp.Body.Close()
		break
	}

	return nil
}

func extractPageID(link string) string {
	parts := strings.Split(link, "-")
	if len(parts) == 0 {
		return ""
	}
	id := parts[len(parts)-1]
	id = strings.ReplaceAll(id, "-", "")

	if len(id) == 32 {
		// Format as UUID
		return fmt.Sprintf("%s-%s-%s-%s-%s", id[:8], id[8:12], id[12:16], id[16:20], id[20:])
	}

	return ""
}
