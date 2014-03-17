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
	c.debug("Joing Chord DHT using source node %v:%v", ip, port)
	// get our successor in the network
	address := ip + ":" + strconv.Itoa(port)
	succReq := NewMessage(NODE_JOIN, c.self.Id, nil)
	resp, err := c.sendToIP(address, succReq)
	if err != nil {
		return err
	} else if resp.purpose == STATUS_ERROR {
		return errors.New(string(resp.value))
	}
	var succ *Node
	err = json.Unmarshal(resp.value, &succ)
	if err != nil {
		return err
	}
	c.self.successor = succ

	// Get our successors predecessor
	predReq := c.NewMessage(PRED_REQ, succ, nil)
	resp, err = c.sendToIP(succ.Host, predReq)
	if err != nil {
		return err
	} else if resp.purpose == STATUS_ERROR {
		return errors.New(string(resp.value))
	}
	var pred *Node
	err = json.Unmarshal(resp.value, &pred)
	if err != nil {
		return err
	}
	c.self.predecessor = pred

	// Notify the specified successor of our existence
	resp, err = c.notify(c.self.successor)
	if err != nil {
		return err
	} else if resp.purpose == STATUS_ERROR {
		return errors.New(string(resp.value))
	}

	return nil
}

// Send a message through the network to it's intended Node
func (c *Cluster) Send(msg Message) (*Message, error) {
	c.debug("Sending message with key %v", msg.key)
	// find the appropriate node in our list of known nodes
	target, err := c.Route(msg.key)
	if err != nil {
		c.throwErr(err)
		return err
	}
	if target == c.self {
		if msg.purpose > PRED_REQ {
			c.onDeliver(msg)
		}
		return nil, nil
	}
	// decide if our application permits the message
	forward := c.forward(msg, target)

	// send the message
	resp, err := c.sendToIP(target.Host, msg)
	if err != nil {
		c.throwErr(err)
		return nil, err
	} else if resp.purpose == STATUS_ERROR {
		c.throwErr(err)
		return resp, errors.New(string(resp.value))
	}

	return resp
}

// Find the appropriate node for the given ID (of the nodes we know about)
func (c *Cluster) Route(key NodeID) (*Node, error) {
	c.debug("Determining route to the given NodeID: %v", key)
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
	c.debug("Recieved a new connection")
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
		var resp *Message
		resp, err := c.onNodeJoin(msg)
		if err != nil {
			resp = c.statusErrMessage(msg.sender, err)
			c.throwErr(err)
		}
		encoder.Encode(resp)
		break

	// A node wants to leave the network
	// so update our finger table accordingly
	case NODE_LEAVE:
		break

	// Recieved a heartbeat message from a connected
	// client, let them know were still alive
	case HEARTBEAT:
		resp := c.onHeartBeat(msg)
		encoder.Encode(heartbeatMsg)
		break

	// We've been notified of a new predecessor
	// node, so update our fingerTable
	case NODE_NOTIFY:
		var resp *Message
		resp, err := c.onNotify(msg)
		if err != nil {
			resp = c.statusErrMessage(msg.sender, err)
			c.throwErr(err)
		}
		encoder.Encode(resp)
		break

	// nil
	//case NODE_ANN:
	//	break

	// Recieved a successor request from a node,
	case SUCC_REQ:
		var resp *Message
		resp, err := c.onSuccessorRequest(msg)
		if err != nil {
			resp = c.statusErrMessage(msg.sender, err)
			c.throwErr(err)
		}
		encoder.Encode(resp)
		break

	// Recieved a predecessor request from a node
	case PRED_REQ:
		var resp *Message
		resp, err := c.onPredecessorRequest(msg)
		if err != nil {
			resp = c.statusErrMessage(msg.sender, err)
			c.throwErr(err)
		}
		encoder.Encode(resp)
		break

	// Not an internal message, forward or deliver it
	default:
		resp := c.statusOKMessage(msg.sender)
		encoder.Encode(resp)
		c.Send(msg)

	}
}

// Send Heartbeats to connected conns (in the cache or finger?)
func (c *Cluster) sendHeartbeats() {
	c.debug("Sending heartbeats...")
	// iterate over the cached conns, and send heartbeat signals,
	// if there's no response then remove it from the cache
	// we're using the lfucache EvictIf function because it gives us the
	// ability to iterate over all the items in the cache (connections)
	// will look into a better cache iteration method later.
	c.connCache.EvictIf(func(tempSock interface{}) bool {
		sock := tempSock.(*sock)

		// Craft a heartbeat message, send and listen for the resp,
		// if there's no response remove from cache (and finger?)
		heartbeat := c.NewMessage(NODE_HEARTBEAT, nil, nil)
		ack, err := c.sendToIP(sock.host, heartbeat)
		if err != nil {
			c.debug("Removing cached node %v", sock.host)
			return true
		}

		// Immediately notify cache NOT to delete this item for now (so we can run these in parallel)
		return false
	})

	// Should I also iterate over the finger table?
	// for now no...
}

// Send a message to a Specific IP in the network, block for messsage?
func (c *Cluster) sendToIP(addr string, msg *Message) (*Message, error) {
	c.debug("Sending message %v to address %v", msg.key, addr)
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
	var resp *Message
	err = sock.read(resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
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

//////////////////////////
//						//
// Application handlers //
//						//
//////////////////////////

func (c *Cluster) onDeliver(msg *Message) {
	c.debug("Delivering message to registered applications")
}

func (c *Cluster) onHeartBeat(msg *Message) *Message {
	c.debug("Recieved heartbeat message from node %v", msg.sender)
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

/*func (c *Cluster) onApp(appFn func(Application)) {
	for _, app := range c.apps {
		appFn(app)
	}
}*/

// Handle any cluster errors
func (c *Cluster) throwErr(err error) {
	c.err(err.Error())
	// Send the error through all the embedded apps
	for _, app := range c.apps {
		app.OnError(err)
	}
}
