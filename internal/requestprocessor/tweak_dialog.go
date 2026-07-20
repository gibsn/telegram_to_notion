package requestprocessor

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	tweakConversationTTL     = 10 * time.Minute
	tweakCallbackPrefix      = "tweak:"
	tweakTrackCallbackPrefix = "twtrk:"
	telegramCallbackDataMax  = 64
)

type tweakAction string

const (
	tweakActionDemo   tweakAction = "demo"
	tweakActionMix    tweakAction = "mix"
	tweakActionRender tweakAction = "render"
	tweakActionToWork tweakAction = "towork"
)

type tweakConversationKey struct {
	chatID int64
	userID int64
}

type pendingTweak struct {
	action             tweakAction
	trackName          string
	promptMessageID    int
	expiresAt          time.Time
	repliedToText      string
	repliedToEntities  []tgbotapi.MessageEntity
	repliedToMessageID int
}

func newTweakMenuResponse() commandResponse {
	markup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Demo", tweakCallbackPrefix+string(tweakActionDemo)),
			tgbotapi.NewInlineKeyboardButtonData("Mix", tweakCallbackPrefix+string(tweakActionMix)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Render", tweakCallbackPrefix+string(tweakActionRender)),
			tgbotapi.NewInlineKeyboardButtonData("To work", tweakCallbackPrefix+string(tweakActionToWork)),
		),
	)

	return commandResponse{
		text:        "Choose an action for /tweak:",
		replyMarkup: &markup,
	}
}

func newTweakTrackMenuResponse(action tweakAction, cache tracksCache) (commandResponse, error) {
	if cache == nil {
		return commandResponse{}, errors.New("tracks cache is not initialized")
	}

	trackNames := cache.GetTrackNames()
	if len(trackNames) == 0 {
		return commandResponse{text: "No tracks are available."}, nil
	}

	buttons := make([]tgbotapi.InlineKeyboardButton, 0, len(trackNames))
	for _, trackName := range trackNames {
		trackID, ok := cache.GetTrackID(trackName)
		if !ok {
			continue
		}

		callbackData := tweakTrackCallbackPrefix + string(action) + ":" + trackID
		if len(callbackData) > telegramCallbackDataMax {
			return commandResponse{}, fmt.Errorf("track callback data is too long for %q", trackName)
		}
		buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(trackName, callbackData))
	}

	rows := make([][]tgbotapi.InlineKeyboardButton, 0, (len(buttons)+1)/2)
	for len(buttons) > 0 {
		rowLength := min(2, len(buttons))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(buttons[:rowLength]...))
		buttons = buttons[rowLength:]
	}
	markup := tgbotapi.NewInlineKeyboardMarkup(rows...)

	return commandResponse{
		text:        fmt.Sprintf("Choose a track for %s:", tweakActionLabel(action)),
		replyMarkup: &markup,
	}, nil
}

func tweakActionLabel(action tweakAction) string {
	switch action {
	case tweakActionDemo:
		return "Demo"
	case tweakActionMix:
		return "Mix"
	case tweakActionRender:
		return "Render"
	case tweakActionToWork:
		return "To work"
	default:
		return string(action)
	}
}

func parseTweakCallback(data string) (tweakAction, bool) {
	action := tweakAction(strings.TrimPrefix(data, tweakCallbackPrefix))
	if !strings.HasPrefix(data, tweakCallbackPrefix) {
		return "", false
	}

	switch action {
	case tweakActionDemo, tweakActionMix, tweakActionRender, tweakActionToWork:
		return action, true
	default:
		return "", false
	}
}

func parseTweakTrackCallback(data string) (tweakAction, string, bool) {
	if !strings.HasPrefix(data, tweakTrackCallbackPrefix) {
		return "", "", false
	}

	parts := strings.SplitN(strings.TrimPrefix(data, tweakTrackCallbackPrefix), ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", false
	}

	action, ok := parseTweakCallback(tweakCallbackPrefix + parts[0])
	if !ok {
		return "", "", false
	}

	return action, parts[1], true
}

func tweakActionPrompt(action tweakAction) (text, placeholder string) {
	switch action {
	case tweakActionDemo, tweakActionMix:
		return "Send a reply with:\nedit name\n[start [end]]\n[description]", "edit, time, description"
	case tweakActionRender:
		return "Send the iteration number as a reply.", "3"
	default:
		return "", ""
	}
}

func (p *RequestProcessor) processCallbackQuery(callback *tgbotapi.CallbackQuery) {
	if callback == nil || callback.From == nil {
		return
	}

	fromUserName := strings.ToLower(callback.From.UserName)
	if !p.allowedToCreate[fromUserName] {
		p.answerCallback(callback.ID, "You are not allowed to use this command")
		return
	}

	if callback.Message == nil || callback.Message.Chat == nil {
		p.answerCallback(callback.ID, "The original message is unavailable")
		return
	}

	if action, ok := parseTweakCallback(callback.Data); ok {
		p.processTweakActionCallback(callback, action)
		return
	}
	if action, trackID, ok := parseTweakTrackCallback(callback.Data); ok {
		p.processTweakTrackCallback(callback, action, trackID)
		return
	}

	p.answerCallback(callback.ID, "Unknown action")
}

func (p *RequestProcessor) processTweakActionCallback(
	callback *tgbotapi.CallbackQuery,
	action tweakAction,
) {
	response, err := newTweakTrackMenuResponse(action, p.tracksCache)
	if err != nil {
		p.answerCallback(callback.ID, err.Error())
		return
	}

	p.answerCallback(callback.ID, "")
	p.sendCallbackResponse(callback, response)
}

func (p *RequestProcessor) processTweakTrackCallback(
	callback *tgbotapi.CallbackQuery,
	action tweakAction,
	trackID string,
) {
	if p.tracksCache == nil {
		p.answerCallback(callback.ID, "Tracks cache is not initialized")
		return
	}

	trackName, ok := p.tracksCache.GetTrackName(trackID)
	if !ok {
		p.answerCallback(callback.ID, "This track is no longer available")
		return
	}

	p.answerCallback(callback.ID, "")
	if action == tweakActionToWork {
		command := commandCommon{
			command:       "/tweak",
			restOfMessage: "towork " + trackName,
			fromUserName:  strings.ToLower(callback.From.UserName),
			fromUserID:    callback.From.ID,
			isPrivate:     callback.Message.Chat.IsPrivate(),
			chatID:        callback.Message.Chat.ID,
		}
		response, err := withUsageErrorReply(command, "$track", p.processTweakToWorkResponse)
		if err != nil {
			log.Printf("Could not process interactive tweak towork: %v", err)
		}
		p.sendCallbackResponse(callback, response)
		return
	}

	promptText, placeholder := tweakActionPrompt(action)
	if callback.From.UserName != "" {
		promptText = fmt.Sprintf("@%s, %s", callback.From.UserName, promptText)
	}
	promptText += "\n\nSend /cancel to cancel."

	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, promptText)
	msg.ReplyToMessageID = callback.Message.MessageID
	msg.ReplyMarkup = tgbotapi.ForceReply{
		ForceReply:            true,
		InputFieldPlaceholder: placeholder,
		Selective:             callback.From.UserName != "",
	}

	sent, err := p.bot.Send(msg)
	if err != nil {
		log.Printf("Could not send tweak prompt to Telegram: %v", err)
		return
	}

	pending := pendingTweak{
		action:          action,
		trackName:       trackName,
		promptMessageID: sent.MessageID,
	}
	if commandMessage := callback.Message.ReplyToMessage; commandMessage != nil &&
		commandMessage.ReplyToMessage != nil {
		originalMessage := commandMessage.ReplyToMessage
		pending.repliedToText = originalMessage.Text
		pending.repliedToEntities = originalMessage.Entities
		pending.repliedToMessageID = originalMessage.MessageID
	}

	p.setPendingTweak(callback.Message.Chat.ID, callback.From.ID, pending)
}

func (p *RequestProcessor) sendCallbackResponse(
	callback *tgbotapi.CallbackQuery,
	response commandResponse,
) {
	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, response.text)
	msg.ParseMode = "HTML"
	msg.ReplyToMessageID = callback.Message.MessageID
	if response.replyMarkup != nil {
		msg.ReplyMarkup = *response.replyMarkup
	}

	if _, err := p.bot.Send(msg); err != nil {
		log.Printf("Could not send callback response to Telegram: %v", err)
	}
}

func (p *RequestProcessor) answerCallback(callbackID, text string) {
	if _, err := p.bot.Request(tgbotapi.NewCallback(callbackID, text)); err != nil {
		log.Printf("Could not answer Telegram callback: %v", err)
	}
}

func (p *RequestProcessor) setPendingTweak(chatID, userID int64, pending pendingTweak) {
	p.pendingTweaksMu.Lock()
	defer p.pendingTweaksMu.Unlock()

	now := p.now()
	for key, current := range p.pendingTweaks {
		if !current.expiresAt.After(now) {
			delete(p.pendingTweaks, key)
		}
	}

	pending.expiresAt = now.Add(tweakConversationTTL)
	p.pendingTweaks[tweakConversationKey{chatID: chatID, userID: userID}] = pending
}

func (p *RequestProcessor) takePendingTweak(message *tgbotapi.Message) (pendingTweak, bool, bool) {
	if message == nil || message.Chat == nil || message.From == nil || message.ReplyToMessage == nil {
		return pendingTweak{}, false, false
	}

	key := tweakConversationKey{chatID: message.Chat.ID, userID: message.From.ID}

	p.pendingTweaksMu.Lock()
	defer p.pendingTweaksMu.Unlock()

	pending, ok := p.pendingTweaks[key]
	if !ok || pending.promptMessageID != message.ReplyToMessage.MessageID {
		return pendingTweak{}, false, false
	}

	delete(p.pendingTweaks, key)
	if !pending.expiresAt.After(p.now()) {
		return pendingTweak{}, true, true
	}

	return pending, true, false
}

func (p *RequestProcessor) processPendingTweakReply(
	message *tgbotapi.Message,
) (commandResponse, error) {
	pending, found, expired := p.takePendingTweak(message)
	if !found {
		return commandResponse{}, errNotACommand
	}
	if expired {
		return commandResponse{text: "This action has expired. Send /tweak again."}, nil
	}

	command := pendingTweakCommand(pending, message)

	switch pending.action {
	case tweakActionDemo, tweakActionMix:
		text, err := withErrorReply(command, p.processTweak)
		return commandResponse{text: text}, err
	case tweakActionRender:
		return withUsageErrorReply(
			command,
			"$track $iteration_number",
			p.processTweakRenderResponse,
		)
	case tweakActionToWork:
		return withUsageErrorReply(command, "$track", p.processTweakToWorkResponse)
	default:
		return commandResponse{}, errors.New("unknown pending tweak action")
	}
}

func pendingTweakCommand(pending pendingTweak, message *tgbotapi.Message) commandCommon {
	separator := " "
	if pending.action == tweakActionDemo || pending.action == tweakActionMix {
		separator = "\n"
	}
	restOfMessage := string(pending.action) + " " + pending.trackName +
		separator + strings.TrimSpace(message.Text)

	return commandCommon{
		command:            "/tweak",
		restOfMessage:      restOfMessage,
		repliedToText:      pending.repliedToText,
		repliedToEntities:  pending.repliedToEntities,
		fromUserName:       strings.ToLower(message.From.UserName),
		fromUserID:         message.From.ID,
		isPrivate:          message.Chat.IsPrivate(),
		chatID:             message.Chat.ID,
		repliedToMessageID: pending.repliedToMessageID,
	}
}

func (p *RequestProcessor) processCancel(message commandCommon) string {
	key := tweakConversationKey{chatID: message.chatID, userID: message.fromUserID}

	p.pendingTweaksMu.Lock()
	defer p.pendingTweaksMu.Unlock()

	if _, ok := p.pendingTweaks[key]; !ok {
		return "There is no active action."
	}

	delete(p.pendingTweaks, key)
	return "Action cancelled."
}
