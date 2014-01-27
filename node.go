package ghord

import (
	"net"
)

// Node
//	Host		string
//	ID			[]byte
// 	lock sync.Mutex
//	successor	*Node
//	predessor	*Node
//	fingers		fingerTable

// Remote Node
//	Node (Embedded Struct)
//	transport transport

type Node struct {
	Id          NodeID
	Host        string
	Port        int
	successor   *Node
	predecessor *Node
}

func NewNode(id NodeID, host string, port int) *Node {
	node := &Node{Id: id, Host: host, Port: port}
	return node
}
