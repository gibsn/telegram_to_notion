package notion

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path"
	"sort"
	"strings"
)

// Tweak status constants for demo tweaks
const (
	TweakDemoStatusTODO = "todo"
)

// Tweak status constants for mix tweaks
const (
	TweakMixStatusAnalysis     = "Анализ"
	TweakMixStatusDeferred     = "Отложено"
	TweakMixStatusReadyForWork = "Готово к работе"
	TweakMixStatusInWork       = "В работе"
)

// Track status constants for "In progress" group
const (
	TrackStatusDemo      = "Демка"
	TrackStatusRecording = "Запись"
	TrackStatusMixing    = "Сведение"
)

// createTracksInProgressFilter creates a filter for tracks with "In progress" statuses
func createTracksInProgressFilter() map[string]interface{} {
	inProgressStatuses := []string{
		TrackStatusDemo,
		TrackStatusRecording,
		TrackStatusMixing,
	}

	orFilters := make([]map[string]interface{}, 0, len(inProgressStatuses))
	for _, status := range inProgressStatuses {
		orFilters = append(orFilters, map[string]interface{}{
			"property": "Статус",
			"status": map[string]string{
				"equals": status,
			},
		})
	}

	return map[string]interface{}{
		"or": orFilters,
	}
}

type TrackPage struct {
	Title  string
	PageID string
	Link   string
}

func trackLinkFromPageID(pageID string) string {
	return notionURL + strings.ReplaceAll(pageID, "-", "")
}

func parseTrackPages(result struct {
	Results []struct {
		ID         string `json:"id"`
		Properties map[string]struct {
			Title []struct {
				PlainText string `json:"plain_text"`
			} `json:"title"`
		} `json:"properties"`
	} `json:"results"`
}) []TrackPage {
	tracks := make([]TrackPage, 0, len(result.Results))

	for _, r := range result.Results {
		prop, ok := r.Properties["Название"]
		if !ok || len(prop.Title) == 0 {
			continue
		}

		tracks = append(tracks, TrackPage{
			Title:  prop.Title[0].PlainText,
			PageID: r.ID,
			Link:   trackLinkFromPageID(r.ID),
		})
	}

	sort.Slice(tracks, func(i, j int) bool {
		return strings.ToLower(tracks[i].Title) < strings.ToLower(tracks[j].Title)
	})

	return tracks
}

func (n *Notion) loadTrackPages(dbID string, filter map[string]interface{}) ([]TrackPage, error) {
	payload := map[string]interface{}{}
	if filter != nil {
		payload["filter"] = filter
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("could not marshal json payload: %w", err)
	}

	req, err := http.NewRequest(
		"POST", n.apiBaseURL+path.Join("databases", dbID, "query"),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create a request: %w", err)
	}

	if n.debug {
		url := n.apiBaseURL + path.Join("databases", dbID, "query")
		log.Printf("Tracks url: %s", url)
	}

	resp, err := n.doWithRetries(req, body)
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

	return parseTrackPages(result), nil
}

func (n *Notion) LoadTrackPages(dbID string) ([]TrackPage, error) {
	return n.loadTrackPages(dbID, createTracksInProgressFilter())
}

func (n *Notion) LoadAllTrackPages(dbID string) ([]TrackPage, error) {
	return n.loadTrackPages(dbID, nil)
}

// LoadTracks queries the tracks database and returns a list of track titles (property "Название")
// Only returns tracks with status from "In progress" group: Демка, Запись, Сведение
func (n *Notion) LoadTracks(dbID string) (map[string]string, error) {
	tracks, err := n.LoadTrackPages(dbID)
	if err != nil {
		return nil, err
	}

	titlesToIDs := make(map[string]string, len(tracks))
	for _, track := range tracks {
		titlesToIDs[track.Title] = track.PageID
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

type RenderTweak struct {
	ID          string
	Summary     string
	TrackPart   string
	Start       string
	End         string
	Explanation string
	Author      string
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
		log.Printf("Tweaks payload: %s", string(prettyPayload))
	}

	req, err := http.NewRequest("POST", n.apiBaseURL+"pages", nil)
	if err != nil {
		return "", fmt.Errorf("could not create a request: %w", err)
	}

	resp, err := n.doWithRetries(req, body)
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

func (n *Notion) LoadReadyMixTweaksForTrack(trackPageID string) ([]RenderTweak, error) {
	pages, err := n.loadReadyMixTweakPagesForTrack(trackPageID)
	if err != nil {
		return nil, err
	}

	tweaks := make([]RenderTweak, 0, len(pages))
	for _, page := range pages {
		tweak := parseRenderTweak(page.Properties)
		tweak.ID = page.ID
		tweaks = append(tweaks, tweak)
	}
	return tweaks, nil
}

func (n *Notion) CountUnreadyMixTweaksForTrack(trackPageID string) (int, error) {
	pages, err := n.loadUnreadyMixTweakPagesForTrack(trackPageID)
	if err != nil {
		return 0, err
	}

	return len(pages), nil
}

func (n *Notion) MoveReadyMixTweaksToWorkForTrack(trackPageID string) (int, error) {
	pages, err := n.loadReadyMixTweakPagesForTrack(trackPageID)
	if err != nil {
		return 0, err
	}

	for _, page := range pages {
		if err := n.setMixTweakStatus(page.ID, TweakMixStatusInWork); err != nil {
			return 0, fmt.Errorf("failed to update tweak %s status: %w", page.ID, err)
		}
	}

	return len(pages), nil
}

type mixTweakPage struct {
	ID         string                    `json:"id"`
	Properties map[string]notionProperty `json:"properties"`
}

func (n *Notion) loadReadyMixTweakPagesForTrack(trackPageID string) ([]mixTweakPage, error) {
	return n.loadMixTweakPagesForTrack(trackPageID, "equals", TweakMixStatusReadyForWork)
}

func (n *Notion) loadUnreadyMixTweakPagesForTrack(trackPageID string) ([]mixTweakPage, error) {
	statusFilters := make([]map[string]interface{}, 0, 2)
	for _, status := range []string{TweakMixStatusAnalysis, TweakMixStatusDeferred} {
		statusFilters = append(statusFilters, map[string]interface{}{
			"property": "Статус",
			"status": map[string]string{
				"equals": status,
			},
		})
	}

	return n.loadMixTweakPagesForTrackWithStatusFilter(trackPageID, map[string]interface{}{
		"or": statusFilters,
	})
}

func (n *Notion) loadMixTweakPagesForTrack(
	trackPageID string,
	statusFilterOperator string,
	status string,
) ([]mixTweakPage, error) {
	return n.loadMixTweakPagesForTrackWithStatusFilter(trackPageID, map[string]interface{}{
		"property": "Статус",
		"status": map[string]string{
			statusFilterOperator: status,
		},
	})
}

func (n *Notion) loadMixTweakPagesForTrackWithStatusFilter(
	trackPageID string,
	statusFilter map[string]interface{},
) ([]mixTweakPage, error) {
	if n.tweaksMixDBID == "" {
		return nil, fmt.Errorf("tweaks mix DB ID is not set")
	}
	if strings.TrimSpace(trackPageID) == "" {
		return nil, fmt.Errorf("track page ID is empty")
	}

	payload := map[string]interface{}{
		"filter": map[string]interface{}{
			"and": []map[string]interface{}{
				{
					"property": "Песня",
					"relation": map[string]string{
						"contains": trackPageID,
					},
				},
				statusFilter,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("could not marshal json payload: %w", err)
	}

	req, err := http.NewRequest(
		"POST", n.apiBaseURL+path.Join("databases", n.tweaksMixDBID, "query"), nil,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create a request: %w", err)
	}

	resp, err := n.doWithRetries(req, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Results []mixTweakPage `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Results, nil
}

func (n *Notion) setMixTweakStatus(pageID, status string) error {
	if strings.TrimSpace(pageID) == "" {
		return fmt.Errorf("tweak page ID is empty")
	}

	payload := map[string]interface{}{
		"properties": map[string]interface{}{
			"Статус": map[string]interface{}{
				"status": map[string]string{
					"name": status,
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("could not marshal request: %w", err)
	}

	req, err := http.NewRequest("PATCH", n.apiBaseURL+path.Join("pages", pageID), nil)
	if err != nil {
		return fmt.Errorf("could not create a request: %w", err)
	}

	resp, err := n.doWithRetries(req, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

type notionProperty struct {
	Title    []plainTextPart `json:"title"`
	RichText []plainTextPart `json:"rich_text"`
	Select   *struct {
		Name string `json:"name"`
	} `json:"select"`
	Status *struct {
		Name string `json:"name"`
	} `json:"status"`
	People []struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"people"`
}

type plainTextPart struct {
	PlainText string `json:"plain_text"`
}

func parseRenderTweak(props map[string]notionProperty) RenderTweak {
	return RenderTweak{
		Summary:     propertyText(props, "Кратко"),
		TrackPart:   propertyText(props, "Дорожка"),
		Start:       propertyText(props, "Начало интервала"),
		End:         propertyText(props, "Конец интервала"),
		Explanation: propertyText(props, "Пояснение"),
		Author:      firstNonEmpty(propertyText(props, "Автор"), propertyText(props, "Автор (Manual)")),
	}
}

func propertyText(props map[string]notionProperty, name string) string {
	prop, ok := props[name]
	if !ok {
		return ""
	}

	switch {
	case len(prop.Title) > 0:
		return plainText(prop.Title)
	case len(prop.RichText) > 0:
		return plainText(prop.RichText)
	case prop.Select != nil:
		return prop.Select.Name
	case prop.Status != nil:
		return prop.Status.Name
	case len(prop.People) > 0:
		names := make([]string, 0, len(prop.People))
		for _, person := range prop.People {
			if person.Name != "" {
				names = append(names, person.Name)
			} else if person.ID != "" {
				names = append(names, person.ID)
			}
		}
		return strings.Join(names, ", ")
	default:
		return ""
	}
}

func plainText(parts []plainTextPart) string {
	var b strings.Builder
	for _, part := range parts {
		b.WriteString(part.PlainText)
	}
	return b.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
