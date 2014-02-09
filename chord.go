package ghord

import (
	"crypto/sha1"
	"hash"
)

var (
	// Hashing function to use for all chord IDs and utilities
	hasher hash.Hash = sha1.New()
)

// Chord Struct

//Config Struct

//Application Handler/Callback/Delegate
type Application interface {
	// When a error occurs in the functionality of the DHT
	OnError(err error)

	// Recieved a message intended for the self node
	OnDeliver(msg Message)

	// Recieved a message that needs to be routed onwards
	OnForward(msg *Message, node *Node) bool // return False if Wendy should not forward

	// Added a new successor to our figerTable
	//OnNewFinger(leafset []*Node)

	// A new node has joined the network
	OnNodeJoin(node Node)

	// A node has left the network
	OnNodeExit(node Node)

	// Recieved a heartbeat signal from a peer
	OnHeartbeat(node Node)
}

// Global Encoder/ Decoder

// Set the hashing algorithm to use for internal use of NodeId generation
// IMPORTANT: Must be set BEFORE any Node or Cluster Objects are created!! If done afterwards you will most certainly break
// the functionality of the DHT.
// The default hashing algorithm is Sha1
func SetHash(h hash.Hash) {
	hasher = h
}
