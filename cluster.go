package ghord

import (
	"encoding/json"
	"errors"
	"log"
	"log/syslog"
	"math/rand"
	"net"
	"strconv"
	"time"

	"github.com/calmh/lfucache"
)

//Cluster Struct
//	self 			*Node
//	kill			chan bool
//	app				Application/Callback/Delegate
//	log				logger
//	logLevel		int
//	stabalizeMin	time.Duration
//	stabalizeMax	time.Duration
//	numSuccessors	int
//	joined			bool
//	credentials		credentials
//	transport		transport
// 	heartbeat

//NewCluster
// Initiate a new cluster with the given node
// initiate the logger with a log level
// set the rest of the settings with the given config

type Cluster struct {
	self          *Node
	kill          chan bool
	apps          []Application
	log           log
	logLevel      int
	fingerTable   fingerTable
	numFingers    int
	stabilizeMin  time.Duration
	stabilizeMax  time.Duration
	heartbeatFreq time.Duration
	connTimeout   time.Duration
	joined        bool
	creds         credentials
	//transport     transport

	connenctions chan net.Conn
	cacheSize    int
	connCache    *lfucache.Cache
}

// A configuration template for a cluster
type ClusterOptions struct {
	StabilizeMin time.Duration
	StabilizeMax time.Duration
	Log          log
	credentials  Credentials
}

var (
	logger, _ = syslog.New(syslog.LOG_ERR, "[GHORD]")
)

// Create a new Cluster
func NewCluster(n *Node, options ...ClusterOptions) *Cluster {
	return &Cluster{
		self: n,
		kill: make(chan bool),
		log, logger,
		logLevel, syslog.LOG_ERR,
		stabilizeMin: time.Second * 5,
		stabilizeMax: time.Second * 10,
		heartbeat:    time.Second * 10,
		connTimeout:  time.Second * 30,
		numFingers:   hasher.Size() * 8,
		connnections: make(chan net.Conn),
		cacheSize:    (hasher.Size() * 8) / 2,
		connCache:    lfucache.New((hasher.Size() * 8) / 2),
	}
}

// Updates configuration for Cluter, Can only be used before cluster is started via either Start() or Join()
func (c *Cluster) Config(config) {}

// Public API

// Start the cluster, listen and participate in the network
func (c *Cluster) Listen() error {
	portStr := strconv.Itoa(c.self.Port)
	addr := c.self.Host + ":" + portStr

	c.debug("Listening on %v", addr)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	defer ln.Close()

	go func(ln net.Listener, conns chan net.Conn) {
		for {
			conn, err := ln.Accept()
			if err != nil {
				c.throwErr(err)
				return
			}
			c.debug("Recieved connections")
			conns <- conn
		}
	}(ln, c.connections)

	// Also add stabilization ticker
	for {
		select {
		case <-c.kill:
			return nil
		case <-time.After(c.heartbeatFreq):
			c.debug("Sending heartbeats")
			go c.sendHeartbeats()
			break
		// Run the stabilize routine randomly between stabilizeMin and stabilizeMax
		case <-time.After(time.Duration(randRange(c.stabilizeMin.Nanoseconds(), c.stabilizeMax.Nanoseconds()))):
			c.debug("Running stabilize routine")
			c.stabilize()
		case conn := <-connections:
			c.debug("Handling connection")
			go c.handleConn(conn)
			break
		}
	}

	return nil
}

//Stop the cluster
func (c *Cluster) Stop() {
	c.debug("Killing cluster")
	c.kill <- true

	//Notify the cluster that we're leaving the network
}

// Join the network, using a node known to be on the network identified by ip:port
func (c *Cluster) Join(ip string, port int) error {

	// get our successor in the network
	address := ip + ":" + strconv.Itoa(port)
	getSuccMsg := NewMessage(NODE_JOIN, c.self.Id, nil)
	recvSuccMsg, err := c.sendToIP(address, getSuccMsg)

	// decode the message body, the node representing our predecessor
	var succ *Node
	err = json.Unmarshal(recvSuccMsg.value, succ)
	if err != nil {
		return err
	}
	c.self.successor = succ

	//Notify the rest of the network of our existence
	err = c.notify(c.self.successor)
	if err != nil {
		return err
	}

	return nil
}

// Send a message through the network to it's intended Node
func (c *Cluster) Send(msg Message) (*Message, error) {
	// find the appropriate node in our list of known nodes
	target, err := c.Route(msg.key)
	if err != nil {
		return err
	}
	if target == c.self {
		if msg.purpose > PRED_REQ {
			c.deliver(msg)
		}
		return nil, nil
	}
	// decide if our application permits the message
	forward := c.forward(msg, target)

	// send the message
	recv, err := c.sendToIP(target.Host, msg)
	if err != nil {
		c.throwErr(err)
	}

	return recv
}

// Find the appropriate node for the given ID (of the nodes we know about)
func (c *Cluster) Route(key NodeID) (*Node, error) {
	// check if we are responsible for the key
	if betweenRightInc(c.self.predecessor.Id, c.self.Id, key) {
		c.debug("I'm the target. Delievering message %v", key)
		return c.self, nil
	}

	// check if our successor is responsible for the key
	if betweenRightInc(c.self.Id, c.self.successor.Id, key) {
		// our successor is the desired node
		c.debug("Our successor is the target. Delievering message %s", key)
		return c.self.successor
	}

	// finally check if one our fingers is closer to (or is) the desired node
	c.debug("Checking fingerTable for target node...")
	return c.closestPreccedingNode(key)
}

// Internal methods //

// Handle new connections
func (c *Cluster) handleConn(conn net.Conn) {
	defer conn.Close()
	var msg Message
	decoder := NewDecoder(conn)
	encoder := NewEncoder(conn)
	if err := decoder.Decode(&msg); err != nil {
		c.throwErr(err)
		return
	}
	msg.hops++
	c.debug("Recieved a message with purpose %v from node %v", msg.purpose, msg.sender)

	switch msg.purpose {

	// A node wants to join the network
	// we need to find his appropriate successor
	case NODE_JOIN:
		succReq := c.NewMessage(SUCC_REQ, msg.sender, nil)
		resp, err := c.Send(succReq)
		if err != nil {
			errMsg := c.statusErrMessage(msg.sender, err)
			encoder.Encode(errMsg)
			c.throwErr(err)
			c.err(err.Error())
			break
		}

		// respond with the found successor
		encoder.Encode(resp)
		break

	// A node wants to leave the network
	// so update our finger table accordingly
	case NODE_LEAVE:
		break

	// Recieved a heartbeat message from a connected
	// client, let them know were still alive
	case HEARTBEAT:
		heartbeatMsg := c.NewMessage(STATUS_OK, msg.sender, nil)
		encoder.Encode(heartbeatMsg)
		break

	// We've been notified of a new predecessor
	// node, so update our fingerTable
	case NODE_NOTIFY:
		var predecessor *Node
		err := json.Unmarshal(msg.value, &predecessor)
		if err != nil {
			errMsg := c.statusErrMessage(msg.sender, err)
			encoder.Encode(errMsg)
			c.throwErr(err)
			c.err(err.Error())
			break
		}
		break

	// nil
	case NODE_ANN:
		break

	// Recieved a successor request from a node,
	case SUCC_REQ:
		resp, err := c.onSuccessorRequest(msg)
		if err != nil {
			c.throwErr(err)
			c.err(err.Error())
		}
		encoder.Encode(resp)
		break

	// Recieved a predecessor request from a node
	case PRED_REQ:
		break

	// Not an internal message, forward it on
	default:
		c.Send(msg)
	}
}

// Send Heartbeats to connected conns (in the cache or finger?)
func (c *Cluster) sendHeartbeats() {

	// iterate over the cached conns, and send heartbeat signals,
	// if there's no response then remove it from the cache
	// we're using the lfucache EvictIf function because it gives us the
	// ability to iterate over all the items in the cache (connections)
	// will look into a better cache iteration method later.
	c.connCache.EvictIf(func(tempSock interface{}) {
		go func() {
			sock := tempSock.(*sock)

			// Craft a heartbeat message, send and listen for the resp,
			// if there's no response remove from cache (and finger?)
			heartbeat := c.NewMessage(NODE_HEARTBEAT, nil, nil)
			ack, err := c.sendToIP(sock.host, heartbeat)
			if err != nil {
				c.connCache.Delete(sock.host)
			}
		}()

		// Immediately notify cache NOT to delete this item for now (so we can run these in parallel)
		return false
	})

	// Should I also iterate over the finger table?
	// for now no...
}

// Send a message to a Specific IP in the network, block for messsage?
func (c *Cluster) sendToIP(addr string, msg Message) (*Message, error) {
	sock, err := c.getSock(addr)
	if err != nil {
		return nil, err
	}
	defer c.putSock(sock)

	// Send the message to the connected peer
	err = sock.write(msg)
	if err != nil {
		return nil, err
	}

	// read the response
	var recvMsg *Message
	err = sock.read(recvMsg)
	if err != nil {
		return nil, err
	}

	return recvMsg, nil
}

// Chord API Method Calls //

// CHORD API - Find the first successor for the given ID
func (c *Cluster) findSuccessor(key NodeID) (*Node, error) {
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
	// craft message for pred_req
	predReq := c.NewMessage(PRED_REQ, c.self.successor.Id, nil)
	resp, err := c.sendToIP(c.self.successor.Host, predReq)

	if err != nil {
		return err
	} else if predReq.purpose == ERROR {
		err = errors.New("Error durring stabilize routine... Couldnt get predecessor request")
		c.err(err)
		c.throwErr(err.Error())
	}

	// Decode response into a node obj
	var predecessor *Node
	err = json.Unmarshal(resp.value, &predecessor)

	// check if our sucessor has a diff predecessor then us
	// if so notify the new successor and update our own records
	if c.self.Id != predecessor.Id {
		c.debug("Found new predecessor! Id: %v", predecessor.Id)
		notifyReq := c.NewMessage(NODE_NOTIFY, predecessor.Id, nil)
		resp, err := c.sendToIP(predecessor.Host, notifyReq)
		if err != nil {
			return err
		}

		// update our own successor
		c.self.successor = predecessor
	}

	return nil
}

// CHORD API - Notify a Node of our existence
func (c *Cluster) notify(n *Node) {
	c.debug("Notifying node: %v", n.Id)
	ann := c.NewMessage(NODE_ANN, n.Id, nil)
	c.Send(ann)

	// Is there more?
}

// CHORD API - fix fingers
func (c *Cluster) fixFingers() {}

//////////////////////////
//						//
// Application handlers //
//						//
//////////////////////////

func (c *Cluster) onDeliver(msg *Message) {}

func (c *Cluster) onHeartBeat(msg *Message) *Message {}

func (c *Cluster) onNodeJoin(msg *Message) {}

func (c *Cluster) onNodeLeave(msg *Message) {}

func (c *Cluster) onNotify(msg *Message) {}

func (c *Cluster) onSuccessorRequest(msg *Message) (*Message, error) {}

func (c *Cluster) onPredecessorRequest(msg *Message) (*Message, error) {}

// Decide wheather or not to continue forwarding the message through the network
func (c *Cluster) forward(msg *Message, next *Node) bool {
	c.debug("Checking if we should forward the given message")
	forward = true
	for _, app := range c.apps {
		forward &= app.OnForward(msg, next)
	}

	return forward
}

// Handle any cluster errors
func (c *Cluster) throwErr(err error) {
	// Send the error through all the embedded apps
	for _, app := range c.apps {
		app.OnError(err)
	}

	//c.debug(err.Error())
	c.err(err.Error())
}
