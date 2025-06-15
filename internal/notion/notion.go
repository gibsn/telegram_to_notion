package notion

import (
	"net/http"
	"time"
)

type Notion struct {
	token string

	client *http.Client
}

const (
	timeoutNotionAPI = 10 * time.Second
)

const (
	notionURL = "https://www.notion.so/"
	notionAPI = "https://api.notion.com/v1/"
)

func NewNotion(token string) *Notion {
	n := &Notion{
		token: token,
		client: &http.Client{
			Timeout: timeoutNotionAPI,
		},
	}

	return n
}
