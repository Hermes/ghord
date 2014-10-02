package ghord

import (
	"bytes"
	"encoding/json"
	"errors"
)

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
		return nil, errors.New(string(response.body))
	}

	var node *Node
	err = json.Unmarshal(response.body, &node)
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
	prev := c.self
	for _, finger := range c.fingerTable.table {
		if betweenRightInc(prev.Id, finger.node.Id, key) {
			return finger.node, nil
		}
		prev = finger.node
	}

	return nil, errors.New("No node exists for given id")
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
		errStr := string(resp.body)
		return errors.New(errStr)
	}

	var predecessor *Node
	err = c.Decode(bytes.NewBuffer(resp.body), predecessor)

	// check if our sucessor has a diff predecessor then us
	// if so notify the new successor and update our own records
	if c.self.Id.Equal(predecessor.Id) {
		c.debug("Found new predecessor! Id: %v", predecessor.Id)
		resp, err := c.notify(predecessor)
		if err != nil {
			return err
		} else if resp.purpose == STATUS_ERROR {
			return errors.New(string(resp.body))
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
