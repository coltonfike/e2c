package e2c

import (
	"time"
)

type ProgressTimer struct {
	Timer *time.Timer
	end   time.Time
}

func NewProgressTimer(t time.Duration) *ProgressTimer {
	return &ProgressTimer{time.NewTimer(t), time.Now().Add(t)}
}

func (pt *ProgressTimer) Reset(t time.Duration) {
	pt.Timer.Reset(t)
	pt.end = time.Now().Add(t)
}

func (pt *ProgressTimer) AddDuration(t time.Duration) {
	d := time.Until(pt.end) + t
	pt.Timer.Reset(d)
	pt.end = time.Now().Add(d)
}
