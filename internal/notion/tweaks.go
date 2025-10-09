package notion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path"
	"strings"
)

// Tweak status constants
const (
	TweakStatusTODO = "todo"
)

// LoadTracks queries the tracks database and returns a list of track titles (property "Песня")
func (n *Notion) LoadTracks(dbID string) (map[string]string, error) {
	payload := map[string]interface{}{}

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
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	if n.debug {
		url := notionAPI + path.Join("databases", dbID, "query")
		log.Println(url)
	}

	resp, err := n.doWithRetries(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Results []struct {
			ID         string `json:"id"`
			Properties map[string]struct {
				Title []struct {
					PlainText string `json:"plain_text"`
				} `json:"title"`
			} `json:"properties"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	titlesToIDs := make(map[string]string, len(result.Results))
	for _, r := range result.Results {
		prop, ok := r.Properties["Название"]
		if !ok || len(prop.Title) == 0 {
			continue
		}
		titlesToIDs[prop.Title[0].PlainText] = r.ID
	}

	return titlesToIDs, nil
}

type CreateTweakDemoRequest struct {
	NotionDBID       string
	TitleProperty    string
	Title            string
	TrackName        string
	TrackPageID      string
	Start            string
	End              string
	Explanation      string
	AuthorNotionUser string

	Debug bool
}

func (n *Notion) CreateTweakDemo(r *CreateTweakDemoRequest) (string, error) {
	// Title property is always "Кратко"
	r.TitleProperty = "Кратко"

	payload := map[string]interface{}{
		"parent": map[string]string{"database_id": r.NotionDBID},
		"properties": map[string]interface{}{
			r.TitleProperty: map[string]interface{}{
				"title": []map[string]interface{}{
					{"text": map[string]string{"content": r.Title}},
				},
			},
			"Статус": map[string]interface{}{
				"select": map[string]string{
					"name": TweakStatusTODO,
				},
			},
		},
	}

	if r.Explanation != "" {
		payload["properties"].(map[string]interface{})["Пояснение"] = map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"text": map[string]string{"content": r.Explanation}},
			},
		}
	}

	if r.TrackPageID != "" {
		payload["properties"].(map[string]interface{})["Песня"] = map[string]interface{}{
			"relation": []map[string]string{{"id": r.TrackPageID}},
		}
	}

	// Optional properties
	props := payload["properties"].(map[string]interface{})

	if r.Start != "" {
		props["Начало интервала"] = map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"text": map[string]string{"content": r.Start}},
			},
		}
	}
	if r.End != "" {
		props["Конец интервала"] = map[string]interface{}{
			"rich_text": []map[string]interface{}{
				{"text": map[string]string{"content": r.End}},
			},
		}
	}
	if r.AuthorNotionUser != "" {
		props["Автор (Manual)"] = map[string]interface{}{
			"people": []map[string]string{
				{
					"object": "user",
					"id":     r.AuthorNotionUser,
				},
			},
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("could not marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", notionAPI+"pages", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("could not create a request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+n.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Notion-Version", "2022-06-28")

	resp, err := n.doWithRetries(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	cleanID := strings.ReplaceAll(result.ID, "-", "")
	url := notionURL + cleanID
	return url, nil
}
