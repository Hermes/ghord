package ghord

import (
	"bytes"
	"encoding/hex"
	"math/big"
)

// Represents a Hash algorithm
type Hasher interface {
	Hash(data []byte) []byte
	Size() int
}

// Represents a NodeID in the form of a hash
// Unless you know what you are doing, do not create a
// NodeID yourself, always use NodeIDFromBytes().
type NodeID []byte

// Create a hashed NodeID from a given byte array
func (c *Cluster) NodeIDFromBytes(id []byte) NodeID {
	return NodeID(c.hasher.Hash(id))
}

func (n NodeID) String() string {
	return hex.EncodeToString(n)
}

// Add a integer to the NodeID.
func (n NodeID) Add(i *big.Int) NodeID {
	newVal := big.NewInt(0)
	y := big.NewInt(0)
	y.SetBytes(n)

	return NodeID(newVal.Add(y, i).Bytes())
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
func (n NodeID) Equal(id NodeID) bool {
	return bytes.Compare(n, id) == 0
}
