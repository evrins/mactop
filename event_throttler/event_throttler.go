package event_throttler

import "time"

type EventThrottler struct {
	timer       *time.Timer
	gracePeriod time.Duration

	C chan struct{}
}

func NewEventThrottler(gracePeriod time.Duration) *EventThrottler {
	return &EventThrottler{
		timer:       nil,
		gracePeriod: gracePeriod,
		C:           make(chan struct{}, 1),
	}
}

func (e *EventThrottler) Notify() {
	if e.timer != nil {
		return
	}

	e.timer = time.AfterFunc(e.gracePeriod, func() {
		e.timer = nil
		select {
		case e.C <- struct{}{}:
		default:
		}
	})
}
