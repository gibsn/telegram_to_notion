package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
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
		return fmt.Errorf("invalid task link %s", setRequest.TaskLink)
	}

	payload := map[string]interface{}{
		"properties": map[string]interface{}{
			"Дедлайн": map[string]interface{}{
				"date": map[string]string{
					"start": setRequest.Deadline.Format("2006-01-02"),
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

func extractPageID(link string) string {
	parts := strings.Split(link, "/")
	lastPart := parts[len(parts)-1]

	idParts := strings.Split(lastPart, "-")
	id := idParts[len(idParts)-1]

	id = strings.ReplaceAll(id, "-", "")

	if len(id) == 32 {
		// Format as UUID
		return fmt.Sprintf("%s-%s-%s-%s-%s", id[:8], id[8:12], id[12:16], id[16:20], id[20:])
	}

	return ""
}
