package backoff

import (
	"context"
	"github.com/clarkmcc/backoff/internal/clock"
	"sync"
	"testing"
	"time"
)

func TestExponentialBackoffManager_next(t *testing.T) {
	c := clock.NewFakeClock(time.Unix(0, 0))

	backoff := NewManager(&Exponential{
		Duration:   time.Second,
		Factor:     2,
		Steps:      10,
		MaxRetries: 2,
	})
	backoff.Clock = c

	var wg sync.WaitGroup

	// Run the test, we should wait one second this time
	timer, done := backoff.next()
	if done {
		t.Fatal("expected done to be false")
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-timer.C()
	}()

	c.Step(time.Second)
	wg.Wait()

	// Run it again, this time we should be waiting two seconds
	timer, done = backoff.next()
	if done {
		t.Fatal("expected done to be false")
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-timer.C()
	}()

	c.Step(2 * time.Second)
	wg.Wait()

	// Advance the clock to the max
	c.Step(time.Minute)
	_, done = backoff.next()
	if !done {
		t.Fatal("expected done to be true")
	}
}

func TestExponentialBackoffManager_Next(t *testing.T) {
	ctx := context.Background()
	backoff := NewManager(&Exponential{
		Duration:   time.Millisecond,
		Factor:     2,
		Steps:      10,
		MaxRetries: 3,
	})

	i := 0
	for backoff.Next(ctx) {
		i++
	}
	if i != 3 {
		t.Fatalf("expected three iterations, got %v", i)
	}
}
