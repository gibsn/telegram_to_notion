package pinger

import (
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	"github.com/gibsn/telegram_to_notion/internal/notion"
	"github.com/gibsn/telegram_to_notion/internal/requestprocessor"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type clock interface {
	Now() time.Time
	Sleep(d time.Duration)
	Until(t time.Time) time.Duration
}

type realClock struct{}

func (realClock) Now() time.Time                  { return time.Now() }
func (realClock) Sleep(d time.Duration)           { time.Sleep(d) }
func (realClock) Until(t time.Time) time.Duration { return time.Until(t) }

type taskCache interface {
	Tasks() []notion.Task
}

type sendPingCB func(chatID int64, mention string, task notion.Task, t time.Time) error

type Pinger struct {
	debug bool

	clock clock

	tasksCache taskCache

	tg            *tgbotapi.BotAPI
	namesResolver *requestprocessor.UserResolver
	chatID        int64
	pingText      string

	startingTime, endTime time.Time
	threshold, period     time.Duration

	sendPingFunc sendPingCB
}

func NewPinger(
	c taskCache, tg *tgbotapi.BotAPI,
	chatID int64,
) (*Pinger, error) {
	loc := time.Now().Location()

	p := &Pinger{
		clock:         realClock{},
		tasksCache:    c,
		tg:            tg,
		startingTime:  time.Date(0, 0, 0, 8, 0, 0, 0, loc),
		endTime:       time.Date(0, 0, 0, 23, 0, 0, 0, loc),
		threshold:     72 * time.Hour,
		period:        4 * time.Hour,
		chatID:        chatID,
		pingText:      "Hi, what's the estimate?",
		namesResolver: requestprocessor.NewUserResolver(),
	}

	p.sendPingFunc = p.sendPing

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

// PingPeriodically runs a daily loop that sends task pings at set times.
//
// Pings start from the first tick after p.startingTime and repeat every period,
// stopping at endTime (e.g., 23:00). After that, the function waits until the
// next day and restarts the same schedule.
//
// Only tasks with a non-zero deadline and a deadline within the threshold
// (i.e., deadline - now <= threshold) are pinged.
//
// Example:
//
//	startingTime = 08:00, period = 4h, endTime = 23:00, threshold = 24h
//	If now = 2025-06-13T07:30 → first ping at 08:00
//	Pings at 08:00, 12:00, 16:00, 20:00 (if deadline is within 24h)
//	Then wait until 2025-06-14T08:00 and repeat
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

		p.pingThroughDay()

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

func (p *Pinger) pingThroughDay() {
	now := p.clock.Now()
	loc := now.Location()

	nightTime := time.Date(
		now.Year(), now.Month(), now.Day(),
		p.endTime.Hour(), 0, 0, 0, loc,
	)

	for {
		now = p.clock.Now()

		if !now.Before(nightTime) {
			return
		}

		log.Println("Sending pings now")

		for _, task := range p.tasksCache.Tasks() {
			if task.Deadline.IsZero() || task.Deadline.Sub(now) > p.threshold {
				continue
			}

			resolvedAssignees := make([]string, 0, len(task.Assignees))

			for _, a := range task.Assignees {
				resolved := p.namesResolver.NotionToTg(a.ID)
				if resolved == "" {
					log.Printf("Could not resolve user ID '%s' from Notion to telegram name", a.ID)
					log.Printf("Skipping ping for task '%s'", task.Title)
					continue
				}

				resolvedAssignees = append(resolvedAssignees, resolved)
			}

			mention := strings.Join(resolvedAssignees, ", ")

			log.Printf("Sending ping for task '%s' to '%s'", task.Title, mention)

			if err := p.sendPingFunc(p.chatID, mention, task, now); err != nil {
				log.Printf(
					"Could not send ping on task '%s' to user '%s': %v",
					task.Title, mention, err,
				)
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

func (p *Pinger) SetThreshold(th time.Duration) {
	p.threshold = th
}

func (p *Pinger) SetStartingTime(t string) error {
	loc := time.Now().Location()

	startAt, err := time.ParseInLocation("15:04", t, loc)
	if err != nil {
		return fmt.Errorf("invalid start time: %w", err)
	}

	p.startingTime = startAt

	return nil
}

func (p *Pinger) SetEndTime(t string) error {
	loc := time.Now().Location()

	endAt, err := time.ParseInLocation("15:04", t, loc)
	if err != nil {
		return fmt.Errorf("invalid end time: %w", err)
	}

	p.endTime = endAt

	return nil
}

func (p *Pinger) SetPeriod(d time.Duration) {
	p.period = d
}

func (p *Pinger) SetPingText(text string) {
	p.pingText = text
}

func (p *Pinger) setClock(clock clock) {
	p.clock = clock
}

func (p *Pinger) setSendPingFunc(f sendPingCB) {
	p.sendPingFunc = f
}
