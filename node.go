package ghord

import ()

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

func NewNode(id NodeID, host string) *Node {
	node := &Node{ID: id, Host: host}
	return node
}
