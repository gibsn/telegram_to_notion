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

// Tweak status constants for demo tweaks
const (
	TweakDemoStatusTODO = "todo"
)

// Tweak status constants for mix tweaks
const (
	TweakMixStatusAnalysis = "Анализ"
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

type CreateTweakRequest struct {
	Title            string
	TrackName        string
	TrackPageID      string
	Start            string
	End              string
	Explanation      string
	AuthorNotionUser string
	StatusType       string // "select" or "status"
}

func (n *Notion) createTweak(dbID, status string, r *CreateTweakRequest) (string, error) {
	if dbID == "" {
		return "", fmt.Errorf("database ID is empty")
	}

	titleProperty := "Кратко"

	// Status field can be either "select" or "status" type
	var statusField map[string]interface{}
	if r.StatusType == "status" {
		statusField = map[string]interface{}{
			"status": map[string]string{
				"name": status,
			},
		}
	} else {
		statusField = map[string]interface{}{
			"select": map[string]string{
				"name": status,
			},
		}
	}

	payload := map[string]interface{}{
		"parent": map[string]string{"database_id": dbID},
		"properties": map[string]interface{}{
			titleProperty: map[string]interface{}{
				"title": []map[string]interface{}{
					{"text": map[string]string{"content": r.Title}},
				},
			},
			"Статус": statusField,
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

	if n.debug {
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

func (n *Notion) CreateTweakDemo(r *CreateTweakRequest) (string, error) {
	if n.tweaksDemoDBID == "" {
		return "", fmt.Errorf("tweaks demo DB ID is not set")
	}
	r.StatusType = "select"
	return n.createTweak(n.tweaksDemoDBID, TweakDemoStatusTODO, r)
}

func (n *Notion) CreateTweakMix(r *CreateTweakRequest) (string, error) {
	if n.tweaksMixDBID == "" {
		return "", fmt.Errorf("tweaks mix DB ID is not set")
	}
	r.StatusType = "status"
	return n.createTweak(n.tweaksMixDBID, TweakMixStatusAnalysis, r)
}
