package ghord

import (
	"log"
	"log/syslog"
	"net"
	"strconv"
	"time"
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
	fingerTable   []*Node
	numFingers    int
	stabilizeMin  time.Duration
	stabilizeMax  time.Duration
	heartbeatFreq time.Duration
	connTimeout   time.Duration
	joined        bool
	creds         credentials
	//transport     transport

	connenctions chan net.Conn
	connCache    sockPool
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
		connCache:    newSockPool(),
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

// Join the network
func (c *Cluster) Join(ip string, port int) error {
	//Initialize the pred/succ + finger table
	sock, err := c.makeSock(ip, port)
	if err != nil {
		return err
	}
	defer c.connCache.putSock(sock.host, sock)

	getSuccMsg := NewMessage(SUCC_REQ, nil, empty)
	err = sock.write(getSuccMsg)
	if err != nil {
		return err
	}
	var recvSuccMsg *Message
	err = sock.read(recvSuccMsg)
	if err != nil {
		return err
	}

	// LAST POINT OF WORK

	//Notify the rest of the network of our existence
}

// Create a new message to be routed through the network
func (c *Cluster) NewMessage(purpose int, key NodeID, body []byte) Message {}

// Send a message through the network to it's intended Node
func (c *Cluster) Send(msg Message) err {}

// Find the appropriate node for the given ID
func (c *Cluster) Route(key NodeID) (*Node, error) {}

// Internal methods //

// Handle new connections
func (c *Cluster) handleConn(conn net.Conn) {}

// Create a new connection to a node, and add it to the sock pool
func (c *Cluster) makeSock(ip string, port int) (*sock, error) {
	address := ip + ":" + strconv.Itoa(port)
	conn, err := net.DialTimeout("tcp", address, c.connTimeout)
	if err != nil {
		c.error("Couldnt get tcp conn to node: %v", err)
		return nil, err
	}

	return newSock(conn), nil
}

// Send Heartbeats to connected conns
func (c *Cluster) sendHeartbeats() {}

// Send a message to a Specific IP in the network
func (c *Cluster) sendToIP(hostname string, msg Message) {}

// API Method Calls //

// CHORD API - Find the first successor for the given ID
func (c *Cluster) findSuccessor(key NodeID) (*Node, error) {}

// CHORD API - Find the first predecessor for the given ID
func (c *Cluster) findPredeccessor(key NodeID) (*Node, error) {}

// CHORD API - Find the closest preceding node in the finger table
func (c *Cluster) closestPreccedingNode(key NodeID) (*Node, error) {}

// CHORD API - Stabilize the fingerTable
func (c *Cluster) stabilize() {}

// CHORD API - Notify a Node of our existence
func (c *Cluster) notify(n *Node) {}

// Application handlers

// Decide wheather or not to continue forwarding the message through the network
func (c *Cluster) forward(msg Message, tid NodeID) bool {}

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
