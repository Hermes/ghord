package ghord

import (
	"encoding/json"
	"errors"
	"log/syslog"
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
	err = json.Unmarshal(resp.value, succ)
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
	err = json.Unmarshal(resp.value, pred)
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
	if target.Id == c.self.Id {
		if msg.purpose > PRED_REQ {
			c.onDeliver(msg)
		}
		return nil, nil
	}

	// decide if our application permits the message
	if c.forward(msg, target) {
		// send the message
		resp, err := c.sendToIP(target.Host, msg)
		if err != nil {
			c.throwErr(err)
			return nil, err
		} else if resp.purpose == STATUS_ERROR {
			c.throwErr(err)
			return resp, errors.New(string(resp.value))
		}
		return resp, nil
	}
	return nil, nil
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
		return c.self.successor, nil
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

	// replace with heartbeat
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
