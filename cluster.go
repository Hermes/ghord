package ghord

import (
	"encoding/json"
	"errors"
	"log"
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
type ClusterConfig struct {
	StabilizeMin time.Duration
	StabilizeMax time.Duration
	Log          log
	credentials  Credentials
}

var (
	logger, _ = syslog.New(syslog.LOG_ERR, "[HERMES]")
)

// Create a new Cluster
func NewCluster(n *Node) *Cluster {
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
		case <-time.After(time.Duration(c.heartbeatFreq)):
			c.debug("Sending heartbeats")
			go c.sendHeartbeats()
			break
		case conn := <-connections:
			c.debug("Handling connection")
			go c.handleConn(conn)
			break
		}
	}

	return nil
}

// Public API

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
	getSuccMsg := NewMessage(NODE_JOIN, c.self.Id, empty)
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

// Create a new message to be routed through the network
func (c *Cluster) NewMessage(purpose int, key NodeID, body []byte) Message {}

// Send a message through the network to it's intended Node
func (c *Cluster) Send(msg Message) (*Message, error) {
	// find the appropriate node
	target, err := c.Route(msg.key)
	if err != nil {
		return err
	}
	if target != nil {
		if msg.purpose > PRED_REQ {
			c.deliver(msg)
		}
		return nil, nil
	}
	// decide if our application permits the message

	// send the message

	// wait for repsonse

	// return response
}

// Find the appropriate node for the given ID (of the nodes we know about)
func (c *Cluster) Route(key NodeID) (*Node, error) {
	// check if we are responsible for the key
	if between(c.self.predecessor.Id, c.self.Id, key) {
		c.debug("I'm the target. Delievering message %v", key)
		return c.self, nil
	}

	// check if our successor is responsible for the key
	if between(c.self.Id, c.self.successor.Id, key) {
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
func (c *Cluster) handleConn(conn net.Conn) {}

// Create a new connection (or get it from the cache) to a node, and add it to the sock pool
// NOTE: the ...int for port is a hack to get overloading (or something like it) working
func (c *Cluster) getSock(addr string, port ...int) (*sock, error) {

	// Normiliza the address (either given full address as string, or as ip:port components)
	var address string
	if len(port) == 1 {
		address = addr + ":" + strconv.Itoa(port[0])
	} else if len(port) > 1 {
		return nil, errors.New("Malformed address")
	} else {
		address = addr
	}

	tempconn, found := c.connCache.Access(address)
	var conn *net.Conn
	if !found {
		conn, err := net.DialTimeout("tcp", address, c.connTimeout)
		if err != nil {
			c.error("Couldnt get tcp conn to node: %v", err)
			return nil, err
		}
	} else {
		conn = tempconn.(*net.Conn)
	}

	return newSock(conn), nil
}

//Put the sock back on to the conn, (thread safe? ...nope lol)
func (c *Cluster) putSock(addr string, s *sock) {
	s.used = time.Now()
	c.connCache.Insert(addr, s)
}

// Send Heartbeats to connected conns (in the cache or finger?)
func (c *Cluster) sendHeartbeats() {

	// iterate over the cached conns, and send heartbeat signals,
	// if there's no response then remove it from the cache
	// we're using the lfucache EvictIf function because it gives us the
	// ability to iterate over all the items in the cache (connections)
	c.connCache.EvictIf(func(tempSock interface{}) {
		go func() {
			sock := tempSock.(*sock)

			// Craft a heartbeat message, send and listen for the resp,
			// if there's no response remove from cache (and finger?)
			heartbeat := c.NewMessage(NODE_HEARTBEAT, nil, empty)
			ack, err := c.sendToIP(sock.host, heartbeat)
			if err != nil {
				c.connCache.Delete(sock.host)
			}
		}()

		// Notify lfucache NOT to delete this item
		return false
	})

	//Should I also iterate over the finger table?
}

// Send a message to a Specific IP in the network, block for messsage?
func (c *Cluster) sendToIP(addr string, msg Message) (*Message, error) {
	sock, err := c.getSock(addr)
	if err != nil {
		return nil, err
	}
	defer c.putSock(sock.host)

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

// API Method Calls //

// CHORD API - Find the first successor for the given ID
func (c *Cluster) findSuccessor(key NodeID) (*Node, error) {}

// CHORD API - Find the first predecessor for the given ID
func (c *Cluster) findPredeccessor(key NodeID) (*Node, error) {}

// CHORD API - Find the closest preceding node in the finger table
func (c *Cluster) closestPreccedingNode(key NodeID) (*Node, error) {}

// CHORD API - Stabilize successor/predecessor pointers
func (c *Cluster) stabilize() {}

// CHORD API - Notify a Node of our existence
func (c *Cluster) notify(n *Node) {
	ann := c.NewMessage(NODE_ANN, n.Id, empty)
	c.Send(ann)

	// Is there more?
}

// CHORD API - fix fingers
func (c *Cluster) fixFingers() {}

// Application handlers

// Decide wheather or not to continue forwarding the message through the network
func (c *Cluster) forward(msg *Message, next *Node) bool {}

// We are the desired recipient of the message
func (c *Cluster) deliver(msg *Message) {}

// Recieved a heatbeat
func (c *Cluster) onHeartBeat() {}

// Handle any cluster errors
func (c *Cluster) throwErr(err error) {}

// UTILITIES
func (c *Cluster) debug(format string, v ...interface{}) {
	if c.logLevel <= syslog.LOG_DEBUG {
		c.log.Printf(format, v...)
	}
}

func (c *Cluster) warn(format string, v ...interface{}) {
	if c.logLevel <= syslog.LOG_WARNING {
		c.log.Printf(format, v...)
	}
}

func (c *Cluster) err(format string, v ...interface{}) {
	if c.logLevel <= syslog.LOG_ERR {
		c.log.Printf(format, v...)
	}
}
