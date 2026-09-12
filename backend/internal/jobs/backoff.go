package jobs

import (
	"math/rand/v2"
	"time"
)

// DefaultBackoff is how long a job waits before each further attempt. Past the
// end of the table every later attempt waits the last entry.
//
// Minutes, not seconds: the ladder this replaced in the notifier was 1s/2s/4s,
// which burned every attempt inside a circuit breaker's sixty-second open
// window and dead-lettered the task before the provider had a chance to come
// back.
var DefaultBackoff = []time.Duration{
	30 * time.Second,
	2 * time.Minute,
	10 * time.Minute,
	30 * time.Minute,
	1 * time.Hour,
}

// JitterFraction is how far either side of the table entry a delay may land.
//
// A fixed table is a scheduled thundering herd. Every row that failed against
// one provider outage carries the same next attempt to the second, so the
// provider's first moment back up is met by the whole backlog at once, which
// knocks it over again and lines every row up for the next entry in the same
// table. Twenty per cent is enough to spread a backlog across the width of the
// wait without making any single delay surprising: the thirty-second first
// retry lands between twenty-four and thirty-six seconds.
const JitterFraction = 0.2

// Backoff returns how long a job waits before attempt number attempts+1,
// spread by JitterFraction.
//
// attempts is how many have already been spent, so the first failure (attempts
// == 1) waits the first entry. A non-positive count is treated as one rather
// than indexing off the front of the table.
func Backoff(table []time.Duration, attempts int) time.Duration {
	return jittered(base(table, attempts), rand.Float64) //nolint:gosec // G404: this spreads a retry, it is not a secret.
}

// base picks the table entry for an attempt, with no jitter. It is separate so
// a test can pin the entry and the spread independently.
func base(table []time.Duration, attempts int) time.Duration {
	if len(table) == 0 {
		table = DefaultBackoff
	}
	i := attempts - 1
	if i < 0 {
		i = 0
	}
	if i >= len(table) {
		i = len(table) - 1
	}
	return table[i]
}

// jittered spreads d by ±JitterFraction. random returns a value in [0, 1).
func jittered(d time.Duration, random func() float64) time.Duration {
	if d <= 0 {
		return 0
	}
	// random()*2-1 is in [-1, 1), so the result is in
	// [d*(1-f), d*(1+f)) — centred on d rather than biased to one side, which
	// a 0..f spread would be.
	spread := float64(d) * JitterFraction * (random()*2 - 1)
	out := time.Duration(float64(d) + spread)
	if out <= 0 {
		return d
	}
	return out
}
