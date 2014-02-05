package ghord

import (
	"math"
	"math/rand"
)

// calculate offset, id + 2^(exp-1) mod 2^m
func powerOffset(id []byte, exp int) int {

}

// Calculate a random number between n, and m
func randRange(min, max int64) int64 {
	return rand.Int63n(max-min) + min
}

// Checks if key is between id1 and id2 exclusivly
func between(id1, id2, key NodeID) bool {}

// Checks if key E (id1, id2]
func betweenRightInc(id1, id2, key NodeID) bool {}

// Checks if key E [id1, id2)
func betweenLeftInc(id1, id2, key NodeID) bool {}
