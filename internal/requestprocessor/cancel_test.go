package requestprocessor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCancelClearsTelegramKeyboardWithoutAffectingOtherUsers(t *testing.T) {
	for _, active := range []bool{false, true} {
		for _, chatType := range []string{"private", "supergroup"} {
			t.Run(fmt.Sprintf("%s/active=%t", chatType, active), func(t *testing.T) {
				var bot *tgbotapi.BotAPI
				sent := make(chan url.Values, 1)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					require.NoError(t, r.ParseForm())
					w.Header().Set("Content-Type", "application/json")
					var result interface{}
					switch {
					case strings.HasSuffix(r.URL.Path, "/getMe"):
						result = tgbotapi.User{ID: 1, IsBot: true, UserName: "test_bot"}
					case strings.HasSuffix(r.URL.Path, "/getUpdates"):
						result = []tgbotapi.Update{}
						if r.Form.Get("offset") != "2" {
							result = []tgbotapi.Update{{UpdateID: 1, Message: &tgbotapi.Message{
								MessageID: 10,
								From:      &tgbotapi.User{ID: 20, UserName: "gibsn"},
								Chat:      &tgbotapi.Chat{ID: 30, Type: chatType},
								Text:      "/cancel@test_bot",
								Entities:  makeBotCommandEntities("/cancel@test_bot"),
							}}}
						} else {
							bot.StopReceivingUpdates()
						}
					case strings.HasSuffix(r.URL.Path, "/sendMessage"):
						sent <- r.Form
						result = tgbotapi.Message{MessageID: 50}
					default:
						result = true
					}
					require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
						"ok": true, "result": result,
					}))
				}))
				defer server.Close()

				var err error
				bot, err = tgbotapi.NewBotAPIWithAPIEndpoint("test-token", server.URL+"/bot%s/%s")
				require.NoError(t, err)
				p := NewRequestProcessor(nil, "", bot)
				if active {
					p.setPendingInput(30, 20, pendingInput{command: "/task", promptMessageID: 40})
				}
				p.setPendingInput(30, 21, pendingInput{command: "/task", promptMessageID: 41})
				p.ProcessRequests()

				var request url.Values
				select {
				case request = <-sent:
				default:
					t.Fatal("cancel did not send a response")
				}
				assert.Equal(t, "10", request.Get("reply_to_message_id"))
				assert.JSONEq(t, `{"remove_keyboard":true,"selective":true}`, request.Get("reply_markup"))
				if active {
					assert.Equal(t, "Action cancelled.", request.Get("text"))
				} else {
					assert.Equal(t, "There is no active action.", request.Get("text"))
				}
				assert.NotContains(t, p.pendingInputs, conversationKey{chatID: 30, userID: 20})
				assert.Contains(t, p.pendingInputs, conversationKey{chatID: 30, userID: 21})
			})
		}
	}
}
