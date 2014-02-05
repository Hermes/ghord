package ghord

import ()

//
//NOTES
//

// Message types
const (
	NODE_JOIN   = iota // A node is joining the network
	NODE_LEAVE         // A node is leaving the network
	HEARTBEAT          // Heartbeat signal
	NODE_NOTIFY        // Notified of node existense
	NODE_ANN           // A node has been announced
	SUCC_REQ           // A request for a nodes successor
	PRED_REQ           // A request for a nodes predecessor
)

// Represents a message in the DHT network
type Message struct {
	key     NodeID // Message Key
	value   []byte // Content of message
	purpose int    // The purpose of the message
	sender  NodeID // The node who sent the message
	target  NodeID // The targer node of the message
	hops    int    // Number of hops so far taken by the message
}

// Create a new message
func (c *Cluster) NewMessage(purpose int, key NodeID, body []byte) *Message {
	// Sender and Target are filled in by the cluster upon sending the message
	return &Message{
		key:     key,
		value:   body,
		purpose: purpose,
		sender:  c.self.Id,
		hops:    0,
	}
}

// Utilies for creating specific messages (Needed???)
func nodeJoinMessage(key NodeID) *Message {
	return NewMessage(NODE_J, key, empty)
}

func heartBeatMessage(key NodeID) *Message {
	return NewMessage(NODE_HEARTBEAT, key, empty)
}

func notifyMessage(key NodeID) *Message {
	return NewMessage(NODE_NOTIFY, key, empty)
}

// ... etc ...
