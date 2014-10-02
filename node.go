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
	node := &Node{Id: id, Haost: host, Port: port}
	return node
}

// Returns true if this node is responsible for the given NodeID
func (n *Node) IsResponsible(id NodeID) bool {
	return (id.Greater(n.predecessor.Id) && id.Less(n.Id)) || id.Equal(n.Id)
}
