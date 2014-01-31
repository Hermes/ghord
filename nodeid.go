package ghord

import (
	"bytes"
	"encoding/hex"
	"math/big"
)

// Represents a NodeID in the form of a hash
type NodeID []byte

// Create a hashed NodeID from a given byte array
func NodeIDFromBytes(id []byte) NodeID {
	hasher.Reset()
	hasher.Write(id)
	return NodeId(hasher.Sum(nil))
}

func (n NodeID) String() string {
	return hex.EncodeToString(n)
}

// Add a integer to the NodeID.
func (n NodeID) Add(i *big.Int) NodeID {
	newVal := big.NewInt(0)
	y := big.NewInt(0)
	y.SetBytes(n)
	x := big.NewInt(i)

	return NodeID(newVal.Add(x, y).Bytes())
}

// Returns true iff NodeID n < id
func (n NodeID) Less(id NodeID) bool {
	return bytes.Compare(n, id) == -1
}

// Returns true iff NodeID n > id
func (n NodeID) Greater(id NodeID) bool {
	return bytes.Compare(n, id) == 1
}

// Returns true iff NodeID n == id
func (n NodeId) Equal(id NodeID) bool {
	return bytes.Compare(n, id) == 0
}

// Checks if key is between id1 and id2 exclusivly
func between(id1, id2, key NodeID) bool {}

// Checks if key E (id1, id2]
func betweenRightInc(id1, id2, key NodeID) bool {}

// Checks if key E [id1, id2)
func betweenLeftInc(id1, id2, key NodeID) bool {}
