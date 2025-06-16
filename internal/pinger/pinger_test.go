package pinger

import (
	"log"
	"sync"
	"testing"
	"time"

	"github.com/gibsn/telegram_to_notion/internal/notion"
)

type mockClock struct {
	curr time.Time
	last time.Time

	mu sync.Mutex
}

func (m *mockClock) Now() time.Time {
	return m.curr
}

func (m *mockClock) Sleep(d time.Duration) {
	m.mu.Lock()
	m.curr = m.curr.Add(d)
	m.mu.Unlock()

	// if reached an end of the time sequence, hang until test finishes
	if !m.curr.Before(m.last) {
		time.Sleep(1 * time.Second)
	}
}

func (m *mockClock) Until(t time.Time) time.Duration {
	return t.Sub(m.curr)
}

type mockTaskCache struct {
	tasks []notion.Task
}

func (m *mockTaskCache) Tasks() []notion.Task {
	return m.tasks
}

func TestPingPeriodically_Scenarios(t *testing.T) {
	type testcase struct {
		name      string
		start     time.Time
		end       time.Time
		wantPings []string
	}

	tests := []testcase{
		{
			name:  "start before first tick, within one day",
			start: time.Date(2025, 6, 13, 8, 0, 0, 0, time.UTC),
			end:   time.Date(2025, 6, 14, 03, 0, 0, 0, time.UTC),
			wantPings: []string{
				"2025-06-13 09:00:00 +0000 UTC",
				"2025-06-13 15:00:00 +0000 UTC",
				"2025-06-13 21:00:00 +0000 UTC",
			},
		},
		{
			name:  "start after one tick",
			start: time.Date(2025, 6, 13, 14, 0, 0, 0, time.UTC),
			end:   time.Date(2025, 6, 14, 03, 0, 0, 0, time.UTC),
			wantPings: []string{
				"2025-06-13 15:00:00 +0000 UTC",
				"2025-06-13 21:00:00 +0000 UTC",
			},
		},
		{
			name:  "start after last tick",
			start: time.Date(2025, 6, 13, 22, 0, 0, 0, time.UTC),
			end:   time.Date(2025, 6, 15, 03, 0, 0, 0, time.UTC),
			wantPings: []string{
				"2025-06-14 09:00:00 +0000 UTC",
				"2025-06-14 15:00:00 +0000 UTC",
				"2025-06-14 21:00:00 +0000 UTC",
			},
		},
		{
			name:  "two full days",
			start: time.Date(2025, 6, 13, 8, 0, 0, 0, time.UTC),
			end:   time.Date(2025, 6, 15, 03, 0, 0, 0, time.UTC),
			wantPings: []string{
				"2025-06-13 09:00:00 +0000 UTC",
				"2025-06-13 15:00:00 +0000 UTC",
				"2025-06-13 21:00:00 +0000 UTC",
				"2025-06-14 09:00:00 +0000 UTC",
				"2025-06-14 15:00:00 +0000 UTC",
				"2025-06-14 21:00:00 +0000 UTC",
			},
		},
		{
			name:  "ping only tasks within threshold",
			start: time.Date(2025, 6, 13, 7, 30, 0, 0, time.UTC),
			end:   time.Date(2025, 6, 13, 23, 0, 0, 0, time.UTC),
			wantPings: []string{
				"2025-06-13 09:00:00 +0000 UTC",
				"2025-06-13 15:00:00 +0000 UTC",
				"2025-06-13 21:00:00 +0000 UTC",
			},
		},
		{
			name:  "ping tasks after deadline",
			start: time.Date(2025, 6, 14, 7, 30, 0, 0, time.UTC),
			end:   time.Date(2025, 6, 14, 23, 0, 0, 0, time.UTC),
			wantPings: []string{
				"2025-06-14 09:00:00 +0000 UTC",
				"2025-06-14 15:00:00 +0000 UTC",
				"2025-06-14 21:00:00 +0000 UTC",
			},
		},
		{
			name:      "ping no tasks past threshold",
			start:     time.Date(2025, 6, 3, 7, 30, 0, 0, time.UTC),
			end:       time.Date(2025, 6, 4, 23, 0, 0, 0, time.UTC),
			wantPings: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			log.Printf("Running test '%s'", tc.name)

			clock := &mockClock{curr: tc.start, last: tc.end}
			cache := &mockTaskCache{
				tasks: []notion.Task{
					{
						Title: "Test task",
						Assignees: []notion.Assignee{
							{Name: "Kirill Alekseev", ID: "7439e2ca-75f8-4024-b170-620ef7ed08b1"},
						},
						Deadline: time.Date(2025, 6, 14, 0, 0, 0, 0, time.Local),
						Link:     "https://notion.so/task",
					},
				},
			}

			var (
				sentTimes []time.Time
				sentMU    sync.Mutex
			)

			p, err := NewPinger(cache, nil, 0)
			if err != nil {
				t.Fatalf("failed to create pinger: %v", err)
			}

			if err := p.SetStartingTime("09:00"); err != nil {
				log.Fatalf("Could not set up starting time for pinger: %v", err)
			}

			p.SetPeriod(6 * time.Hour)

			p.setClock(clock)
			p.setSendPingFunc(
				func(
					chatID int64, mention string, task notion.Task, t time.Time,
				) error {
					sentMU.Lock()
					sentTimes = append(sentTimes, t)
					sentMU.Unlock()

					return nil
				})

			go func() {
				p.PingPeriodically()
			}()

			time.Sleep(100 * time.Millisecond)

			sentMU.Lock()

			if len(sentTimes) != len(tc.wantPings) {
				t.Fatalf("Expected %d pings, got %d", len(tc.wantPings), len(sentTimes))
			}

			for i, want := range tc.wantPings {
				if sentTimes[i].String() != want {
					t.Errorf("Ping %d: expected %s, got %s", i, want, sentTimes[i])
				}
			}

			sentMU.Unlock()
		})
	}
}
