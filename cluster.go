package ghord

import (
	"bytes"
	"errors"
	stdlog "log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/calmh/lfucache"
	"github.com/hermes/ghord/codec"
	"github.com/hermes/ghord/codec/json"
	"github.com/hermes/ghord/hash/sha1"
	"github.com/op/go-logging"
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
	log           *logging.Logger
	logLevel      logging.Level
	fingerTable   fingerTable
	numFingers    int
	stabilizeMin  time.Duration
	stabilizeMax  time.Duration
	heartbeatFreq time.Duration
	connTimeout   time.Duration
	joined        bool
	//creds         credentials

	// Codec & Hashing Suites
	codec  codec.Codec
	hasher Hasher

	connections chan net.Conn
	cacheSize   int
	connCache   *lfucache.Cache
}

// A configuration template for a cluster
type ClusterOptions struct {
	StabilizeMin time.Duration
	StabilizeMax time.Duration
	Log          *logging.Logger
	//credentials  Credentials
}

func initLogger() {

}

// Create a new Cluster
func NewCluster(self *Node, options ...ClusterOptions) *Cluster {
	hasher := sha1.NewHasher()

	// setup logger
	logger := logging.MustGetLogger("GHORD")
	logBackend := logging.NewLogBackend(os.Stderr, "[GHORD]: ", stdlog.LstdFlags)
	logBackend.Color = true
	syslogBackend, err := logging.NewSyslogBackend("[GHORD]: ")
	if err != nil {
		logger.Fatal(err)
	}
	logging.SetBackend(logBackend, syslogBackend)
	logging.SetLevel(logging.DEBUG, "GHORD")

	return &Cluster{
		self:          self,
		kill:          make(chan bool),
		log:           logger,
		logLevel:      logging.DEBUG,
		stabilizeMin:  time.Second * 5,
		stabilizeMax:  time.Second * 10,
		heartbeatFreq: time.Second * 10,
		connTimeout:   time.Second * 30,
		codec:         json.NewCodec(),
		hasher:        hasher,
		numFingers:    hasher.Size() * 8,
		connections:   make(chan net.Conn),
		//cacheSize:    (hasher.Size() * 8) / 2,
		//connCache:    lfucache.New((hasher.Size() * 8) / 2),
	}
}

// Updates configuration for Cluter, Can only be used before cluster is started via either Start() or Join()
//func (c *Cluster) Config(config) {}

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
			//go c.sendHeartbeats()
		// Run the stabilize routine randomly between stabilizeMin and stabilizeMax
		case <-time.After(time.Duration(randRange(c.stabilizeMin.Nanoseconds(), c.stabilizeMax.Nanoseconds()))):
			c.debug("Running stabilize routine")
			c.stabilize()
		case conn := <-c.connections:
			c.debug("Handling connection")
			go c.handleConn(conn)
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
	address := ip + ":" + strconv.Itoa(port)
	var buf *bytes.Buffer

	// get our successor in the network
	succReq := c.NewMessage(NODE_JOIN, c.self.Id, nil)
	resp, err := c.sendToIP(address, succReq)
	if err != nil {
		return err
	} else if resp.purpose == STATUS_ERROR {
		return errors.New(string(resp.body))
	}

	// parse the successor
	var succ *Node
	buf = bytes.NewBuffer(resp.body)
	err = c.Decode(buf, succ)
	if err != nil {
		return err
	}
	c.self.successor = succ

	// reset buffer for reuse
	buf.Reset()

	// Get our successors predecessor
	predReq := c.NewMessage(PRED_REQ, succ.Id, nil)
	resp, err = c.sendToIP(succ.Host, predReq)
	if err != nil {
		return err
	} else if resp.purpose == STATUS_ERROR {
		return errors.New(string(resp.body))
	}

	// parse the predecessor
	var pred *Node
	buf.Write(resp.body)
	err = c.Decode(buf, pred)
	if err != nil {
		return err
	}
	c.self.predecessor = pred

	// Notify the specified successor of our existence
	resp, err = c.notify(c.self.successor)
	if err != nil {
		return err
	} else if resp.purpose == STATUS_ERROR {
		return errors.New(string(resp.body))
	}

	return nil
}

// Send a message through the network to it's intended Node
func (c *Cluster) Send(msg *Message) (*Message, error) {
	c.debug("Sending message with key %v", msg.key)
	// find the appropriate node in our list of known nodes
	target, err := c.Route(msg.key)
	if err != nil {
		c.throwErr(err)
		return nil, err
	}
	if target.Id.Equal(c.self.Id) {
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
			return resp, errors.New(string(resp.body))
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

	sock := c.newSock(conn)
	defer conn.Close()
	var msg *Message
	var resp *Message
	var err error

	if err := sock.read(msg); err != nil {
		c.throwErr(err)
		return
	}
	c.debug("Recieved a message with purpose %v from node %v", msg.purpose, msg.sender)
	msg.hops++

	switch msg.purpose {
	// A node wants to join the network
	// we need to find his appropriate successor
	case NODE_JOIN:
		resp, err = c.onNodeJoin(msg)
		if err != nil {
			resp = c.statusErrMessage(msg.sender.Id, err)
			c.throwErr(err)
		}

	// A node wants to leave the network
	// so update our finger table accordingly
	case NODE_LEAVE:
		break

	// Recieved a heartbeat message from a connected
	// client, let them know were still alive
	case HEARTBEAT:
		resp = c.onHeartBeat(msg)

	// We've been notified of a new predecessor
	// node, so update our fingerTable
	case NODE_NOTIFY:
		resp, err = c.onNotify(msg)
		if err != nil {
			resp = c.statusErrMessage(msg.sender.Id, err)
			c.throwErr(err)
		}

	// Recieved a successor request from a node,
	case SUCC_REQ:
		resp, err = c.onSuccessorRequest(msg)
		if err != nil {
			resp = c.statusErrMessage(msg.sender.Id, err)
			c.throwErr(err)
		}

	// Recieved a predecessor request from a node
	case PRED_REQ:
		resp, err = c.onPredecessorRequest(msg)
		if err != nil {
			resp = c.statusErrMessage(msg.sender.Id, err)
			c.throwErr(err)
		}

	// Not an internal message, forward or deliver it
	default:
		resp = c.statusOKMessage(msg.sender.Id)
		c.Send(msg)
	}

	sock.write(resp)
}

// Send Heartbeats to connected conns (in the cache or finger?)
/*func (c *Cluster) sendHeartbeats() {
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
}*/ // NEED TO REORGINIZE

// Send a message to a Specific IP in the network, block for messsage?
func (c *Cluster) sendToIP(addr string, msg *Message) (*Message, error) {
	c.debug("Sending message %v to address %v", msg.key, addr)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	sock := c.newSock(conn)

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

func (c *Cluster) Encode(dataBuf *bytes.Buffer, v interface{}) error {
	return c.codec.NewEncoder(dataBuf).Encode(v)
}

func (c *Cluster) Decode(dataBuf *bytes.Buffer, v interface{}) error {
	return c.codec.NewDecoder(dataBuf).Decode(v)
}
