package pinger

import (
	"fmt"
	"html"
	"log"
	"time"

	"github.com/gibsn/telegram_to_notion/internal/notion"
	"github.com/gibsn/telegram_to_notion/internal/requestprocessor"
	"github.com/gibsn/telegram_to_notion/internal/taskscache"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	nightTimeStart = 23
)

type Pinger struct {
	debug bool

	tasksCache *taskscache.Cache

	tg            *tgbotapi.BotAPI
	namesResolver *requestprocessor.UserResolver
	chatID        int64
	pingText      string

	startingTime time.Time
	period       time.Duration
}

func NewPinger(
	c *taskscache.Cache, tg *tgbotapi.BotAPI,
	st string, period time.Duration,
	chatID int64,
) (*Pinger, error) {

	p := &Pinger{
		tasksCache:    c,
		tg:            tg,
		period:        period,
		chatID:        chatID,
		pingText:      "Hi, what's the estimate?",
		namesResolver: requestprocessor.NewUserResolver(),
	}

	loc := time.Now().Location()

	startAt, err := time.ParseInLocation("15:04", st, loc)
	if err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}

	p.startingTime = startAt

	return p, nil
}

// PingPeriodically starts a daily loop that sends periodic pings beginning
// from p.startingTime each day and stopping at night.
//
// The first message is aligned to the next tick based on p.period from
// the starting time. For example, if p.startingTime is 08:00 and p.period
// is 4h, and the program starts at 11:30, the first ping will occur at 12:00.
//
// Each day, the pings resume at the same starting time and repeat every
// p.period until a configured nightTime hour (e.g. 22:00).
func (p *Pinger) PingPeriodically() {
	now := time.Now()
	loc := now.Location()

	firstTick := time.Date(
		now.Year(), now.Month(), now.Day(),
		p.startingTime.Hour(), p.startingTime.Minute(), 0, 0, loc,
	)

	// if the starting time has passed, wait for the next tick
	nextTick := firstTick

	for nextTick.Before(time.Now()) {
		nextTick = nextTick.Add(p.period)
	}

	wait := time.Until(nextTick)
	log.Printf("Waiting until %s to send first message", nextTick.Format(time.RFC1123))
	time.Sleep(wait)

	// loop over days
	for {
		now = time.Now()

		// ping must start every day at the same time
		firstTick = time.Date(
			now.Year(), now.Month(), now.Day(),
			p.startingTime.Hour(), p.startingTime.Minute(), 0, 0, loc,
		)

		if now.Before(firstTick) {
			wait := time.Until(firstTick)
			log.Printf("Waiting until %s to start today's cycle", firstTick.Format(time.RFC1123))
			time.Sleep(wait)
		}

		// stop sending at night and reschedule for the next day
		nightTime := time.Date(
			now.Year(), now.Month(), now.Day(),
			nightTimeStart, 0, 0, 0, loc,
		)

		// start sending pings every period until night begins
		p.pingThroughDay(nightTime)

		tomorrow := tomorrow()
		log.Printf("Waiting until %s", tomorrow)

		time.Sleep(time.Until(tomorrow))
	}
}

func tomorrow() time.Time {
	return ceil(time.Now(), 24*time.Hour)
}

func ceil(t time.Time, d time.Duration) time.Time {
	if d <= 0 {
		return t
	}

	rounded := t.Round(d)
	if rounded.Before(t) {
		return rounded.Add(d)
	}

	return rounded
}
func (p *Pinger) pingThroughDay(nightTime time.Time) {
	ticker := time.NewTicker(p.period)
	defer ticker.Stop()

	for {
		if time.Now().After(nightTime) {
			return
		}

		log.Println("Sending pings now")

		for _, task := range p.tasksCache.Tasks() {
			if task.Deadline.IsZero() {
				continue
			}

			for _, a := range task.Assignees {
				resolved := p.namesResolver.NotionToTg(a.ID)
				if resolved == "" {
					log.Printf(
						"Could not resolve user ID '%s' from Notion to telegram name", a.ID,
					)
					log.Printf("Skipping ping for task '%s'", task.Title)

					continue
				}

				log.Printf("Sending ping for task '%s' to '%s'", task.Title, resolved)

				if err := p.sendPing(p.chatID, resolved, task); err != nil {
					log.Printf(
						"Could not send ping on task '%s' to user '%s': %v",
						task.Title, resolved, err,
					)
				}
			}
		}

		<-ticker.C
	}
}

func (p *Pinger) sendPing(chatID int64, mention string, task notion.Task) error {
	msgText := fmt.Sprintf(
		"%s\n\n%s\n\n<a href=\"%s\">%s</a>\nDeadline: %s",
		p.pingText,
		mention,
		task.Link,
		html.EscapeString(task.Title),
		task.Deadline.Format("2006-01-02"),
	)

	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ParseMode = "HTML"

	_, err := p.tg.Send(msg)
	if err != nil {
		return err
	}

	return nil
}

func (p *Pinger) SetDebug(debug bool) {
	p.debug = debug
}

func (p *Pinger) SetPingText(text string) {
	p.pingText = text
}
