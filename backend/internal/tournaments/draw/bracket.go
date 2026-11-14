// Package draw holds the pure rules of a tournament: how a bracket is laid out,
// how pairs are split into groups, how a padel score is validated, and how a
// group table is ranked. It knows nothing about storage or HTTP, so every rule
// is tested on plain values.
package draw

import "math/bits"

// BracketSize is the number of slots of a single-elimination bracket for n
// pairs: the next power of two, never below two.
func BracketSize(n int) int {
	if n <= 2 {
		return 2
	}
	return 1 << bits.Len(uint(n-1))
}

// Rounds is how many rounds a bracket of the given size has; the last is the
// final.
func Rounds(size int) int {
	return bits.Len(uint(size)) - 1
}

// SeedOrder is the standard placement of seeds 1..size down the first round,
// two by two: with it seeds 1 and 2 can only meet in the final, 1-4 in the
// semifinals at the earliest, and so on. For size 8 it is 1 8 4 5 2 7 3 6.
func SeedOrder(size int) []int {
	order := []int{1}
	for len(order) < size {
		next := make([]int, 0, len(order)*2)
		total := len(order)*2 + 1
		for _, s := range order {
			next = append(next, s, total-s)
		}
		order = next
	}
	return order
}

// Pairing is one first-round match: the two seeds that meet at Position.
// SeedB is zero when SeedA has a bye.
type Pairing struct {
	Position     int
	SeedA, SeedB int
}

// FirstRound lays out the first round for n seeded pairs. Seeds above n do not
// exist, so the top seeds meet nobody and get the byes.
func FirstRound(n int) []Pairing {
	order := SeedOrder(BracketSize(n))
	out := make([]Pairing, len(order)/2)
	for i := range out {
		a, b := order[2*i], order[2*i+1]
		if b > n {
			b = 0
		}
		out[i] = Pairing{Position: i, SeedA: a, SeedB: b}
	}
	return out
}

// Next is where the winner of the match at (round, position) goes: the match of
// the next round and which of its two slots (1 or 2) they fill.
func Next(round, position int) (nextRound, nextPosition, slot int) {
	return round + 1, position / 2, position%2 + 1
}
