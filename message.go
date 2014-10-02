package ghord

import ()

//
//NOTES
//

// Message types
const (
	NODE_JOIN    = iota // A node is joining the network
	NODE_LEAVE          // A node is leaving the network
	HEARTBEAT           // Heartbeat signal
	NODE_NOTIFY         // Notified of node existense
	NODE_ANN            // A node has been announced
	SUCC_REQ            // A request for a nodes successor
	PRED_REQ            // A request for a nodes predecessor
	STATUS_ERROR        // Response indicating an error
	STATUS_OK           // Simple status OK response
)

// Represents a message in the DHT network
type Message struct {
	id      NodeID // Message unique id
	key     NodeID // Message Key
	purpose uint   // The purpose of the message
	origin  Node   // The node from whom the messages originated
	sender  Node   // The node who sent the message
	target  Node   // The targer node of the message
	hops    uint   // Number of hops so far taken by the message
	body    []byte // Content of message
}

// Create a new message
func (c *Cluster) NewMessage(purpose int, key NodeID, body []byte) *Message {
	// Sender and Target are filled in by the cluster upon sending the message
	return &Message{
		key:     key,
		body:    body,
		purpose: purpose,
		sender:  c.self,
		hops:    0,
	}
}

// Get the message key
func (msg *Message) Key() NodeID {
	return msg.key
}

// Get the message body
func (msg *Message) Body() []byte {
	return msg.value
}

// Get the message purpose
func (msg *Message) Purpose() int {
	return msg.purpose
}

// Get the message hops taken
func (msg *Message) Hops() int {
	return msg.hops
}

// Get the message origin node
func (msg *Message) Origin() Node {
	return msg.origin
}

// Get the message target node
func (msg *Message) Target() Node {
	return msg.target
}

// Get the message sender node
func (msg *Message) Sender() Node {
	return msg.sender
}

// Extract the message body into the given value (must be a pointer), using the provided codec
func (msg *Message) DecodeBody(codec codec.Codec, v interface{}) error {
	return codec.NewDecoder(bytes.NewBuffer(msg.body)).Decode(v)
}

// Helper utilies for creating specific messages

func (c *Cluster) nodeJoinMessage(key NodeID) *Message {
	return NewMessage(NODE_J, key, nil)
}

func (c *Cluster) heartBeatMessage(key NodeID) *Message {
	return NewMessage(NODE_HEARTBEAT, key, nil)
}

func (c *Cluster) notifyMessage(key NodeID) *Message {
	return NewMessage(NODE_NOTIFY, key, nil)
}

func (c *Cluster) statusOKMessage(key NodeID) *Message {
	return c.NewMessage(STATUS_OK, key, nil)
}

func (c *Cluster) statusErrMessage(key NodeID, err error) *Message {
	return c.NewMessage(STATUS_ERROR, target, []byte(err.Error()))
}
