package chord

import ()

// Node
//	Host		string
//	ID			[]byte
// 	lock sync.Mutex
//	successor	*Node
//	predessor	*Node
//	fingers		fingerTable

func NewNode(id NodeID, host string) *Node {
	node := &Node{ID: id, Host: host}
	return node
}
