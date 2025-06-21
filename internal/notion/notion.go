package notion

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Notion struct {
	token string

	client *http.Client
}

const (
	timeoutNotionAPI = 10 * time.Second
	numberOfRetries  = 2
)

const (
	notionURL = "https://www.notion.so/"
	notionAPI = "https://api.notion.com/v1/"
)

const (
	StatusNew      = "новая"
	StatusBacklog  = "бэклог"
	StatusDone     = "уже готово"
	StatusArchived = "архивировано"
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

func (n *Notion) doWithRetries(req *http.Request) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)

	for i := 1; i <= numberOfRetries; i++ {
		resp, err = n.client.Do(req)

		if err != nil || resp.StatusCode >= 300 {
			if err != nil {
				log.Printf("request to Notion API failed: %s", err)
			}
			if resp != nil {
				if resp.StatusCode == 400 {
					bodyBytes, err := io.ReadAll(resp.Body)
					if err != nil {
						log.Printf(
							"request to Notion API failed: failed to read response body: %s", err,
						)
					}

					log.Printf(
						"request to Notion API failed with status code 400: %s", string(bodyBytes),
					)
				}
				resp.Body.Close()
			}

			if i < numberOfRetries {
				log.Printf("retrying request to Notion API")
				continue
			}

			if err == nil {
				err = fmt.Errorf("status code is %d", resp.StatusCode)
			}

			return nil, fmt.Errorf("request to Notion API failed: %w", err)
		}

		break
	}

	return resp, nil
}
