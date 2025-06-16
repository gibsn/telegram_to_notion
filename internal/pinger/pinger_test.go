package pinger_test

// TODO remove times

import (
	"log"
	"sync"
	"testing"
	"time"

	"github.com/gibsn/telegram_to_notion/internal/notion"
	"github.com/gibsn/telegram_to_notion/internal/pinger"
)

type MockClock struct {
	curr time.Time
	last time.Time

	times []time.Time
	idx   int
	mu    sync.Mutex
}

func (m *MockClock) Now() time.Time {
	return m.curr
}

func (m *MockClock) Sleep(d time.Duration) {
	m.mu.Lock()
	m.curr = m.curr.Add(d)
	m.mu.Unlock()

	// if reached an end of the time sequence, hang until test finishes
	if !m.curr.Before(m.last) {
		time.Sleep(1 * time.Second)
	}
}

func (m *MockClock) Until(t time.Time) time.Duration {
	return t.Sub(m.curr)
}

type MockTaskCache struct {
	tasks []notion.Task
}

func (m *MockTaskCache) Tasks() []notion.Task {
	return m.tasks
}

func TestPingPeriodically_Scenarios(t *testing.T) {
	type testcase struct {
		name      string
		start     string
		times     []time.Time
		wantPings []string
	}

	tests := []testcase{
		{
			name: "start before first tick, within one day",
			times: []time.Time{
				time.Date(2025, 6, 13, 8, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 13, 9, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 13, 9, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 13, 15, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 13, 21, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 14, 03, 0, 0, 0, time.UTC),
			},
			wantPings: []string{
				"2025-06-13 09:00:00 +0000 UTC",
				"2025-06-13 15:00:00 +0000 UTC",
				"2025-06-13 21:00:00 +0000 UTC",
			},
		},
		{
			name: "start after one tick",
			times: []time.Time{
				time.Date(2025, 6, 13, 14, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 13, 15, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 13, 15, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 13, 21, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 14, 03, 0, 0, 0, time.UTC),
			},
			wantPings: []string{
				"2025-06-13 15:00:00 +0000 UTC",
				"2025-06-13 21:00:00 +0000 UTC",
			},
		},
		{
			name: "start after last tick",
			times: []time.Time{
				time.Date(2025, 6, 13, 22, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 14, 0, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 14, 9, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 14, 15, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 14, 21, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 15, 03, 0, 0, 0, time.UTC),
			},
			wantPings: []string{
				"2025-06-14 09:00:00 +0000 UTC",
				"2025-06-14 15:00:00 +0000 UTC",
				"2025-06-14 21:00:00 +0000 UTC",
			},
		},
		{
			name: "two full days",
			times: []time.Time{
				// Day 1
				time.Date(2025, 6, 13, 8, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 13, 9, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 13, 9, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 13, 15, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 13, 21, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 14, 03, 0, 0, 0, time.UTC),
				// Day 2
				time.Date(2025, 6, 14, 0, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 14, 9, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 14, 9, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 14, 15, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 14, 21, 0, 0, 0, time.UTC),
				time.Date(2025, 6, 15, 03, 0, 0, 0, time.UTC),
			},
			wantPings: []string{
				"2025-06-13 09:00:00 +0000 UTC",
				"2025-06-13 15:00:00 +0000 UTC",
				"2025-06-13 21:00:00 +0000 UTC",
				"2025-06-14 09:00:00 +0000 UTC",
				"2025-06-14 15:00:00 +0000 UTC",
				"2025-06-14 21:00:00 +0000 UTC",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			log.Printf("Running test '%s'", tc.name)

			clock := &MockClock{times: tc.times, curr: tc.times[0], last: tc.times[len(tc.times)-1]}
			cache := &MockTaskCache{
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

			p, err := pinger.NewPinger(cache, nil, "09:00", 6*time.Hour, 0)
			if err != nil {
				t.Fatalf("failed to create pinger: %v", err)
			}

			p.SetClock(clock)
			p.SetSendPingFunc(
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
