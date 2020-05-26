package cargo

import "time"

// Clock for testable times
type Clock struct {
	now func() time.Time
}

// NewClock makes a Clock capable of getting the actual time
func NewClock(now func() time.Time) Clock {
	return Clock{now: now}
}

// Now returns the current time
func (c Clock) Now() time.Time {
	return c.now()
}
