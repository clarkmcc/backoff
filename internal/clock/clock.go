package clock

import (
	"sync"
	"time"
)

// PassiveClock allows for injecting fake or real clocks into code
// that needs to read the current time but does not support scheduling
// activity in the future.
type PassiveClock interface {
	Now() time.Time
	Since(time.Time) time.Duration
}

// Clock allows for injecting fake or real clocks into code that
// needs to do arbitrary things based on time.
type Clock interface {
	PassiveClock
	// After returns the channel of a new Timer.
	// This method does not allow to free/GC the backing timer before it fires. Use
	// NewTimer instead.
	After(d time.Duration) <-chan time.Time
	// NewTimer returns a new Timer.
	NewTimer(d time.Duration) Timer
	// Sleep sleeps for the provided duration d.
	// Consider making the sleep interruptible by using 'select' on a context channel and a timer channel.
	Sleep(d time.Duration)
	// Tick returns the channel of a new Ticker.
	// This method does not allow to free/GC the backing ticker. Use
	// NewTicker from WithTicker instead.
	Tick(d time.Duration) <-chan time.Time
}

// WithTicker allows for injecting fake or real clocks into code that
// needs to do arbitrary things based on time.
type WithTicker interface {
	Clock
	// NewTicker returns a new Ticker.
	NewTicker(time.Duration) Ticker
}

// WithDelayedExecution allows for injecting fake or real clocks into
// code that needs to make use of AfterFunc functionality.
type WithDelayedExecution interface {
	Clock
	// AfterFunc executes f in its own goroutine after waiting
	// for d duration and returns a Timer whose channel can be
	// closed by calling Stop() on the Timer.
	AfterFunc(d time.Duration, f func()) Timer
}

// WithTickerAndDelayedExecution allows for injecting fake or real clocks
// into code that needs Ticker and AfterFunc functionality
type WithTickerAndDelayedExecution interface {
	WithTicker
	// AfterFunc executes f in its own goroutine after waiting
	// for d duration and returns a Timer whose channel can be
	// closed by calling Stop() on the Timer.
	AfterFunc(d time.Duration, f func()) Timer
}

// Ticker defines the Ticker interface.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

var _ = WithTicker(RealClock{})

// RealClock really calls time.Now()
type RealClock struct{}

// Now returns the current time.
func (RealClock) Now() time.Time {
	return time.Now()
}

// Since returns time since the specified timestamp.
func (RealClock) Since(ts time.Time) time.Duration {
	return time.Since(ts)
}

// After is the same as time.After(d).
// This method does not allow to free/GC the backing timer before it fires. Use
// NewTimer instead.
func (RealClock) After(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// NewTimer is the same as time.NewTimer(d)
func (RealClock) NewTimer(d time.Duration) Timer {
	return &realTimer{
		timer: time.NewTimer(d),
	}
}

// AfterFunc is the same as time.AfterFunc(d, f).
func (RealClock) AfterFunc(d time.Duration, f func()) Timer {
	return &realTimer{
		timer: time.AfterFunc(d, f),
	}
}

// Tick is the same as time.Tick(d)
// This method does not allow to free/GC the backing ticker. Use
// NewTicker instead.
func (RealClock) Tick(d time.Duration) <-chan time.Time {
	return time.Tick(d)
}

// NewTicker returns a new Ticker.
func (RealClock) NewTicker(d time.Duration) Ticker {
	return &realTicker{
		ticker: time.NewTicker(d),
	}
}

// Sleep is the same as time.Sleep(d)
// Consider making the sleep interruptible by using 'select' on a context channel and a timer channel.
func (RealClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

// Timer allows for injecting fake or real timers into code that
// needs to do arbitrary things based on time.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
	Reset(d time.Duration) bool
}

var _ = Timer(&realTimer{})

// realTimer is backed by an actual time.Timer.
type realTimer struct {
	timer *time.Timer
}

// C returns the underlying timer's channel.
func (r *realTimer) C() <-chan time.Time {
	return r.timer.C
}

// Stop calls Stop() on the underlying timer.
func (r *realTimer) Stop() bool {
	return r.timer.Stop()
}

// Reset calls Reset() on the underlying timer.
func (r *realTimer) Reset(d time.Duration) bool {
	return r.timer.Reset(d)
}

type realTicker struct {
	ticker *time.Ticker
}

func (r *realTicker) C() <-chan time.Time {
	return r.ticker.C
}

func (r *realTicker) Stop() {
	r.ticker.Stop()
}

var (
	_ = PassiveClock(&FakePassiveClock{})
	_ = WithTicker(&FakeClock{})
)

// FakePassiveClock implements PassiveClock, but returns an arbitrary time.
type FakePassiveClock struct {
	lock sync.RWMutex
	time time.Time
}

// FakeClock implements clock.Clock, but returns an arbitrary time.
type FakeClock struct {
	FakePassiveClock

	// waiters are waiting for the fake time to pass their specified time
	waiters []*fakeClockWaiter
}

type fakeClockWaiter struct {
	targetTime    time.Time
	stepInterval  time.Duration
	skipIfBlocked bool
	destChan      chan time.Time
	fired         bool
	afterFunc     func()
}

// NewFakePassiveClock returns a new FakePassiveClock.
func NewFakePassiveClock(t time.Time) *FakePassiveClock {
	return &FakePassiveClock{
		time: t,
	}
}

// NewFakeClock constructs a fake clock set to the provided time.
func NewFakeClock(t time.Time) *FakeClock {
	return &FakeClock{
		FakePassiveClock: *NewFakePassiveClock(t),
	}
}

// Now returns f's time.
func (f *FakePassiveClock) Now() time.Time {
	f.lock.RLock()
	defer f.lock.RUnlock()
	return f.time
}

// Since returns time since the time in f.
func (f *FakePassiveClock) Since(ts time.Time) time.Duration {
	f.lock.RLock()
	defer f.lock.RUnlock()
	return f.time.Sub(ts)
}

// SetTime sets the time on the FakePassiveClock.
func (f *FakePassiveClock) SetTime(t time.Time) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.time = t
}

// After is the fake version of time.After(d).
func (f *FakeClock) After(d time.Duration) <-chan time.Time {
	f.lock.Lock()
	defer f.lock.Unlock()
	stopTime := f.time.Add(d)
	ch := make(chan time.Time, 1) // Don't block!
	f.waiters = append(f.waiters, &fakeClockWaiter{
		targetTime: stopTime,
		destChan:   ch,
	})
	return ch
}

// NewTimer constructs a fake timer, akin to time.NewTimer(d).
func (f *FakeClock) NewTimer(d time.Duration) Timer {
	f.lock.Lock()
	defer f.lock.Unlock()
	stopTime := f.time.Add(d)
	ch := make(chan time.Time, 1) // Don't block!
	timer := &fakeTimer{
		fakeClock: f,
		waiter: fakeClockWaiter{
			targetTime: stopTime,
			destChan:   ch,
		},
	}
	f.waiters = append(f.waiters, &timer.waiter)
	return timer
}

// AfterFunc is the Fake version of time.AfterFunc(d, cb).
func (f *FakeClock) AfterFunc(d time.Duration, cb func()) Timer {
	f.lock.Lock()
	defer f.lock.Unlock()
	stopTime := f.time.Add(d)
	ch := make(chan time.Time, 1) // Don't block!

	timer := &fakeTimer{
		fakeClock: f,
		waiter: fakeClockWaiter{
			targetTime: stopTime,
			destChan:   ch,
			afterFunc:  cb,
		},
	}
	f.waiters = append(f.waiters, &timer.waiter)
	return timer
}

// Tick constructs a fake ticker, akin to time.Tick
func (f *FakeClock) Tick(d time.Duration) <-chan time.Time {
	if d <= 0 {
		return nil
	}
	f.lock.Lock()
	defer f.lock.Unlock()
	tickTime := f.time.Add(d)
	ch := make(chan time.Time, 1) // hold one tick
	f.waiters = append(f.waiters, &fakeClockWaiter{
		targetTime:    tickTime,
		stepInterval:  d,
		skipIfBlocked: true,
		destChan:      ch,
	})

	return ch
}

// NewTicker returns a new Ticker.
func (f *FakeClock) NewTicker(d time.Duration) Ticker {
	f.lock.Lock()
	defer f.lock.Unlock()
	tickTime := f.time.Add(d)
	ch := make(chan time.Time, 1) // hold one tick
	f.waiters = append(f.waiters, &fakeClockWaiter{
		targetTime:    tickTime,
		stepInterval:  d,
		skipIfBlocked: true,
		destChan:      ch,
	})

	return &fakeTicker{
		c: ch,
	}
}

// Step moves the clock by Duration and notifies anyone that's called After,
// Tick, or NewTimer.
func (f *FakeClock) Step(d time.Duration) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.setTimeLocked(f.time.Add(d))
}

// SetTime sets the time.
func (f *FakeClock) SetTime(t time.Time) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.setTimeLocked(t)
}

// Actually changes the time and checks any waiters. f must be write-locked.
func (f *FakeClock) setTimeLocked(t time.Time) {
	f.time = t
	newWaiters := make([]*fakeClockWaiter, 0, len(f.waiters))
	for i := range f.waiters {
		w := f.waiters[i]
		if !w.targetTime.After(t) {
			if w.skipIfBlocked {
				select {
				case w.destChan <- t:
					w.fired = true
				default:
				}
			} else {
				w.destChan <- t
				w.fired = true
			}

			if w.afterFunc != nil {
				w.afterFunc()
			}

			if w.stepInterval > 0 {
				for !w.targetTime.After(t) {
					w.targetTime = w.targetTime.Add(w.stepInterval)
				}
				newWaiters = append(newWaiters, w)
			}

		} else {
			newWaiters = append(newWaiters, f.waiters[i])
		}
	}
	f.waiters = newWaiters
}

// HasWaiters returns true if After or AfterFunc has been called on f but not yet satisfied (so you can
// write race-free tests).
func (f *FakeClock) HasWaiters() bool {
	f.lock.RLock()
	defer f.lock.RUnlock()
	return len(f.waiters) > 0
}

// Sleep is akin to time.Sleep
func (f *FakeClock) Sleep(d time.Duration) {
	f.Step(d)
}

var _ = Timer(&fakeTimer{})

// fakeTimer implements clock.Timer based on a FakeClock.
type fakeTimer struct {
	fakeClock *FakeClock
	waiter    fakeClockWaiter
}

// C returns the channel that notifies when this timer has fired.
func (f *fakeTimer) C() <-chan time.Time {
	return f.waiter.destChan
}

// Stop stops the timer and returns true if the timer has not yet fired, or false otherwise.
func (f *fakeTimer) Stop() bool {
	f.fakeClock.lock.Lock()
	defer f.fakeClock.lock.Unlock()

	newWaiters := make([]*fakeClockWaiter, 0, len(f.fakeClock.waiters))
	for i := range f.fakeClock.waiters {
		w := f.fakeClock.waiters[i]
		if w != &f.waiter {
			newWaiters = append(newWaiters, w)
		}
	}

	f.fakeClock.waiters = newWaiters

	return !f.waiter.fired
}

// Reset resets the timer to the fake clock's "now" + d. It returns true if the timer has not yet
// fired, or false otherwise.
func (f *fakeTimer) Reset(d time.Duration) bool {
	f.fakeClock.lock.Lock()
	defer f.fakeClock.lock.Unlock()

	active := !f.waiter.fired

	f.waiter.fired = false
	f.waiter.targetTime = f.fakeClock.time.Add(d)

	var isWaiting bool
	for i := range f.fakeClock.waiters {
		w := f.fakeClock.waiters[i]
		if w == &f.waiter {
			isWaiting = true
			break
		}
	}
	if !isWaiting {
		f.fakeClock.waiters = append(f.fakeClock.waiters, &f.waiter)
	}

	return active
}

type fakeTicker struct {
	c <-chan time.Time
}

func (t *fakeTicker) C() <-chan time.Time {
	return t.c
}

func (t *fakeTicker) Stop() {
}
