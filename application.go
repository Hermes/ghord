package ghord

import "encoding/json"

//Application Handler/Callback/Delegate
type Application interface {
	// When a error occurs in the functionality of the DHT
	OnError(err error)

	// Recieved a message intended for the self node
	OnDeliver(msg Message)

	// Recieved a message that needs to be routed onwards
	OnForward(msg *Message, node *Node) bool // return False if ghord should not forward

	// Added a new successor to our figerTable
	//OnNewFinger(leafset []*Node)

	// A new node has joined the network
	OnNodeJoin(node Node)

	// A node has left the network
	OnNodeExit(node Node)

	// Recieved a heartbeat signal from a peer
	OnHeartbeat(node Node)
}

//////////////////////////////////////////////
//											//
//  Internal Cluster Application handlers 	//
//											//
//////////////////////////////////////////////

func (c *Cluster) onDeliver(msg *Message) {
	c.debug("Delivering message to registered applications")
	for _, app := range c.apps {
		app.OnDeliver(msg)
	}
}

func (c *Cluster) onHeartBeat(msg *Message) *Message {
	c.debug("Recieved heartbeat message from node %v", msg.sender)
	for _, app := range c.apps {
		app.OnHeartbeat(msg.Sender())
	}
	return c.NewMessage(STATUS_OK, msg.sender, nil)
}

func (c *Cluster) onNodeJoin(msg *Message) (*Message, error) {
	c.debug("Recieved node join message from node %v", msg.sender)
	req := c.NewMessage(SUCC_REQ, msg.sender, nil)
	return c.Send(req)
}

func (c *Cluster) onNodeLeave(msg *Message) {}

func (c *Cluster) onNotify(msg *Message) (*Message, error) {
	c.debug("Node is notifying us of its existence")
	err := json.Unmarshal(msg.value, &c.self.predecessor)
	if err != nil {
		return c.statusErrMessage(msg.sender, err), err
	}
	return c.statusOKMessage(msg.sender), nil

}

func (c *Cluster) onSuccessorRequest(msg *Message) (*Message, error) {
	c.debug("Recieved successor request from node %v", msg.sender)
	if c.self.IsResponsible(msg.target) {
		// send successor
		succ, err := json.Marshal(c.self.successor)
		if err != nil {
			return c.statusErrMessage(msg.sender, err), ere
		}
		return c.NewMessage(SUCC_REQ, msg.sender, succ), nil
	} else {
		// forward it on
		return c.Send(msg)
	}
}

func (c *Cluster) onPredecessorRequest(msg *Message) (*Message, error) {
	c.debug("Recieved predecessor request from node: %v", msg.sender)
	if c.self.IsResponsible(msg.target) {
		// send successor
		pred, err := json.Marshal(c.self.predecessor)
		if err != nil {
			return c.statusErrMessage(msg.sender, err), err
		}
		return c.NewMessage(PRED_REQ, msg.sender, pred), nil
	} else {
		// forward it on
		return c.Send(msg)
	}
}

// Decide whether or not to continue forwarding the message through the network
func (c *Cluster) forward(msg *Message, next *Node) bool {
	c.debug("Checking if we should forward the given message")
	forward = true

	for _, app := range c.apps {
		forward = forward && app.OnForward(msg, next)
	}

	return forward
}

// Handle any cluster errors
func (c *Cluster) throwErr(err error) {
	c.err(err.Error())
	// Send the error through all the embedded apps
	for _, app := range c.apps {
		app.OnError(err)
	}
}
