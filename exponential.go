package backoff

import (
	"context"
	"github.com/clarkmcc/backoff/internal/clock"
	"math/rand"
	"time"
)

// Exponential holds parameters applied to a Backoff function.
type Exponential struct {
	// The initial duration.
	Duration time.Duration
	// Duration is multiplied by factor each iteration, if factor is not zero
	// and the limits imposed by Steps and Cap have not been reached.
	// Should not be negative.
	// The jitter does not contribute to the updates to the duration parameter.
	Factor float64
	// The sleep at each iteration is the duration plus an additional
	// amount chosen uniformly at random from the interval between
	// zero and `jitter*duration`.
	Jitter float64
	// The remaining number of iterations in which the duration
	// parameter may change (but progress can be stopped earlier by
	// hitting the cap). If not positive, the duration is not
	// changed. Used for exponential backoff in combination with
	// Factor and Cap.
	Steps int
	// A limit on revised values of the duration parameter. If a
	// multiplication by the factor parameter would make the duration
	// exceed the cap then the duration is set to the cap and the
	// steps parameter is set to zero.
	Cap time.Duration
	// The maximum number of times we can exponentially backoff
	Max int
}

// Step (1) returns an amount of time to sleep determined by the
// original Duration and Jitter and (2) mutates the provided Backoff
// to update its Steps and Duration.
func (b *Exponential) Step() time.Duration {
	if b.Steps < 1 {
		if b.Jitter > 0 {
			return Jitter(b.Duration, b.Jitter)
		}
		return b.Duration
	}
	b.Steps--

	duration := b.Duration

	// calculate the next step
	if b.Factor != 0 {
		b.Duration = time.Duration(float64(b.Duration) * b.Factor)
		if b.Cap > 0 && b.Duration > b.Cap {
			b.Duration = b.Cap
			b.Steps = 0
		}
	}

	if b.Jitter > 0 {
		duration = Jitter(duration, b.Jitter)
	}
	return duration
}

// Jitter returns a time.Duration between duration and duration + maxFactor *
// duration.
//
// This allows clients to avoid converging on periodic behavior. If maxFactor
// is 0.0, a suggested default value will be chosen.
func Jitter(duration time.Duration, maxFactor float64) time.Duration {
	if maxFactor <= 0.0 {
		maxFactor = 1.0
	}
	wait := duration + time.Duration(rand.Float64()*maxFactor*float64(duration))
	return wait
}

type ExponentialBackoffManager struct {
	Clock   clock.Clock
	backoff *Exponential // The backoff configuration
	n       int          // The number of retries that have been performed so far
}

// next returns a new timer with the next exponential backoff interval, or returns
// done if no more retries are necessary.
func (e *ExponentialBackoffManager) next() (timer clock.Timer, done bool) {
	e.n++
	if e.backoff.Max > 0 && e.n > e.backoff.Max {
		return nil, true
	}
	return e.Clock.NewTimer(e.backoff.Step()), false
}

func (e *ExponentialBackoffManager) Next(ctx context.Context) bool {
	timer, done := e.next()
	if done {
		return false
	}
	select {
	case <-ctx.Done():
		timer.Stop()
		return false
	case <-timer.C():
		return true
	}
}

func ExponentialBackoff(backoff *Exponential) *ExponentialBackoffManager {
	return &ExponentialBackoffManager{
		Clock:   &clock.RealClock{},
		backoff: backoff,
	}
}
