package pinger

// TODO refactor statuses, comment for ping

import (
	"fmt"
	"html"
	"log"
	"time"

	"github.com/gibsn/telegram_to_notion/internal/notion"
	"github.com/gibsn/telegram_to_notion/internal/requestprocessor"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	nightTimeStart = 23
)

type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
	Until(t time.Time) time.Duration
}

type RealClock struct{}

func (RealClock) Now() time.Time                  { return time.Now() }
func (RealClock) Sleep(d time.Duration)           { time.Sleep(d) }
func (RealClock) Until(t time.Time) time.Duration { return time.Until(t) }

type TaskCache interface {
	Tasks() []notion.Task
}

type sendPingCB func(chatID int64, mention string, task notion.Task, t time.Time) error

type Pinger struct {
	debug bool

	clock Clock

	tasksCache TaskCache

	tg            *tgbotapi.BotAPI
	namesResolver *requestprocessor.UserResolver
	chatID        int64
	pingText      string

	startingTime time.Time
	period       time.Duration

	sendPingFunc sendPingCB
}

func NewPinger(
	c TaskCache, tg *tgbotapi.BotAPI,
	st string, period time.Duration,
	chatID int64,
) (*Pinger, error) {
	p := &Pinger{
		clock:         RealClock{},
		tasksCache:    c,
		tg:            tg,
		period:        period,
		chatID:        chatID,
		pingText:      "Hi, what's the estimate?",
		namesResolver: requestprocessor.NewUserResolver(),
	}

	p.sendPingFunc = p.sendPing

	loc := time.Now().Location()

	startAt, err := time.ParseInLocation("15:04", st, loc)
	if err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}

	p.startingTime = startAt

	return p, nil
}

func (p *Pinger) nextTickAfter() time.Time {
	now := p.clock.Now()
	loc := now.Location()

	firstTick := time.Date(
		now.Year(), now.Month(), now.Day(),
		p.startingTime.Hour(), p.startingTime.Minute(), 0, 0, loc,
	)

	nextTick := firstTick

	for nextTick.Before(now) {
		nextTick = nextTick.Add(p.period)
	}

	return nextTick
}

func (p *Pinger) PingPeriodically() {
	nextTick := p.nextTickAfter()

	log.Printf("Waiting until %s to send first message", nextTick.Format(time.RFC1123))
	p.clock.Sleep(p.clock.Until(nextTick))

	for {
		now := p.clock.Now()
		loc := now.Location()

		firstTick := time.Date(
			now.Year(), now.Month(), now.Day(),
			p.startingTime.Hour(), p.startingTime.Minute(), 0, 0, loc,
		)

		if now.Before(firstTick) {
			wait := p.clock.Until(firstTick)
			log.Printf("Waiting until %s to start today's cycle", firstTick.Format(time.RFC1123))

			p.clock.Sleep(wait)
		}

		nightTime := time.Date(
			now.Year(), now.Month(), now.Day(),
			nightTimeStart, 0, 0, 0, loc,
		)

		p.pingThroughDay(nightTime)

		next := p.tomorrow(now, loc)

		log.Printf("Waiting until %s", next)

		p.clock.Sleep(p.clock.Until(next))
	}
}

func (p *Pinger) tomorrow(startOfDay time.Time, loc *time.Location) time.Time {
	now := p.clock.Now().In(loc)

	dayDelta := 0
	if startOfDay.YearDay() == now.YearDay() {
		dayDelta = 1
	}

	return time.Date(now.Year(), now.Month(), now.Day()+dayDelta, 0, 0, 0, 0, loc)
}

func (p *Pinger) pingThroughDay(nightTime time.Time) {
	for {
		now := p.clock.Now()

		if !now.Before(nightTime) {
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
					log.Printf("Could not resolve user ID '%s' from Notion to telegram name", a.ID)
					log.Printf("Skipping ping for task '%s'", task.Title)
					continue
				}

				log.Printf("Sending ping for task '%s' to '%s'", task.Title, resolved)

				if err := p.sendPingFunc(p.chatID, resolved, task, now); err != nil {
					log.Printf(
						"Could not send ping on task '%s' to user '%s': %v",
						task.Title, resolved, err,
					)
				}
			}
		}

		p.clock.Sleep(p.period)
	}
}

func (p *Pinger) sendPing(chatID int64, mention string, task notion.Task, t time.Time) error {
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

func (p *Pinger) SetClock(clock Clock) {
	p.clock = clock
}

func (p *Pinger) SetSendPingFunc(f sendPingCB) {
	p.sendPingFunc = f
}
