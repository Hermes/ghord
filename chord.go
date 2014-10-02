package ghord

import (
	"encoding/json"
	"errors"
)

// Global Encoder/ Decoder
// Set the hashing algorithm to use for internal use of NodeId generation
// IMPORTANT: Must be set BEFORE any Node or Cluster Objects are created!! If done afterwards you will most certainly break
// the functionality of the DHT.
// The default hashing algorithm is Sha1
func SetHashFunc(h hash.Hash) {
	hasher = h
}

// Chord API Method Calls //

// CHORD API - Find the first successor for the given ID
func (c *Cluster) findSuccessor(key NodeID) (*Node, error) {
	c.debug("Finding successor to key %v", key)
	request := c.NewMessage(SUCC_REQ, key, nil)
	response, err := c.Send(request)
	if err != nil {
		return nil, err
	}

	if response.purpose == STATUS_ERROR {
		return nil, errors.New(string(response.value))
	}

	var node *Node
	err = json.Unmarshal(response.value, &node)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// CHORD API - Find the first predecessor for the given ID
// func (c *Cluster) findPredeccessor(key NodeID) (*Node, error) {}

// CHORD API - Find the closest preceding node in the finger table
func (c *Cluster) closestPreccedingNode(key NodeID) (*Node, error) {
	c.debug("Finding closest node in our finger table to node %v", key)
	for _, finger := range c.fingerTable.table {
		if betweenRightInc(c.self.Id, finger.node.Id, key) {
			return finger.node, nil
		}
		prev = finger.node
	}

	return nil, errors.New("No node exists for id: %s", key)
}

// CHORD API - Stabilize successor/predecessor pointers
func (c *Cluster) stabilize() error {
	c.debug("stabilizing...")
	// craft message for pred_req
	predReq := c.NewMessage(PRED_REQ, c.self.successor.Id, nil)
	resp, err := c.sendToIP(c.self.successor.Host, predReq)

	if err != nil {
		return err
	} else if resp.purpose == STATUS_ERROR {
		var errStr string
		json.Unmarshal(resp.value, &errStr)
		return errors.New(errStr)
	}

	var predecessor *Node
	err = json.Unmarshal(resp.value, &predecessor)

	// check if our sucessor has a diff predecessor then us
	// if so notify the new successor and update our own records
	if c.self.Id != predecessor.Id {
		c.debug("Found new predecessor! Id: %v", predecessor.Id)
		resp, err := c.notify(predecessor)
		if err != nil {
			return err
		} else if resp.purpose == STATUS_ERROR {
			return errors.New(string(resp.value))
		}

		c.self.successor = predecessor
	}

	return nil
}

// CHORD API - Notify a Node of our existence
func (c *Cluster) notify(n *Node) (*Message, error) {
	c.debug("Notifying node: %v of our existence", n.Id)
	ann := c.NewMessage(NODE_NOTIFY, n.Id, nil)
	return c.sendToIP(n.Host, ann)
}

// CHORD API - fix fingers
func (c *Cluster) fixFingers() {}
