package ghord

import (
//"net"
)

// Handles all remoteRPC calls, both incomming and outgoing
// Transport
//	connPool	connPool	// A pool of connections to other nodes in the fingerTable
//	sock 		net.TCPListener		// A TCPListener conn to listen for inbound requests

// Each remote node requires a conn to connect to it
// outConn
//	host	string
//	sock	net.TCPConn
//	enc		Encoder
//	dec		Decoder
//	last	time.Time
