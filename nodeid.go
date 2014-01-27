package ghord

import ()

// Represents a NodeID in the form of a hash
type NodeID []byte

// Create a hashed NodeID from a given byte array
func NodeIDFromBytes(id []byte) NodeID {}

// Checks if key is between id1 and id2 exclusivly
func between(id1, id2, key NodeID) bool {}

// Checks if key E (id1, id2]
func betweenRightInc(id1, id2, key NodeID) bool {}

// Checks if key E [id1, id2)
func betweenLeftInc(id1, id2, key NodeID) bool {}
