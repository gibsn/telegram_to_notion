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
	tweakConversationTTL = 10 * time.Minute
	tweakCallbackPrefix  = "tweak:"
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
		text:        "Выберите действие для /tweak:",
		replyMarkup: &markup,
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

func tweakActionPrompt(action tweakAction) (text, placeholder string) {
	switch action {
	case tweakActionDemo, tweakActionMix:
		return "Отправьте ответом:\nтрек\nназвание правки\n" +
			"[начало [конец]]\n[описание]", "трек, правка, время, описание"
	case tweakActionRender:
		return "Отправьте ответом название трека и номер итерации.", "трек 3"
	case tweakActionToWork:
		return "Отправьте ответом название трека.", "название трека"
	default:
		return "", ""
	}
}

func (p *RequestProcessor) processCallbackQuery(callback *tgbotapi.CallbackQuery) {
	if callback == nil || callback.From == nil {
		return
	}

	action, ok := parseTweakCallback(callback.Data)
	if !ok {
		p.answerCallback(callback.ID, "Неизвестное действие")
		return
	}

	fromUserName := strings.ToLower(callback.From.UserName)
	if !p.allowedToCreate[fromUserName] {
		p.answerCallback(callback.ID, "У вас нет доступа к этой команде")
		return
	}

	p.answerCallback(callback.ID, "")

	if callback.Message == nil || callback.Message.Chat == nil {
		return
	}

	promptText, placeholder := tweakActionPrompt(action)
	if callback.From.UserName != "" {
		promptText = fmt.Sprintf("@%s, %s", callback.From.UserName, promptText)
	}
	promptText += "\n\nДля отмены отправьте /cancel."

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

	pending := pendingTweak{action: action, promptMessageID: sent.MessageID}
	if commandMessage := callback.Message.ReplyToMessage; commandMessage != nil &&
		commandMessage.ReplyToMessage != nil {
		originalMessage := commandMessage.ReplyToMessage
		pending.repliedToText = originalMessage.Text
		pending.repliedToEntities = originalMessage.Entities
		pending.repliedToMessageID = originalMessage.MessageID
	}

	p.setPendingTweak(callback.Message.Chat.ID, callback.From.ID, pending)
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
		return commandResponse{text: "Действие устарело. Снова отправьте /tweak."}, nil
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
	return commandCommon{
		command:            "/tweak",
		restOfMessage:      string(pending.action) + " " + strings.TrimSpace(message.Text),
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
		return "Нет активного действия."
	}

	delete(p.pendingTweaks, key)
	return "Действие отменено."
}
