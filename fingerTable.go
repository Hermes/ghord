package ghord

import (
	"sync"
)

type finger struct {
	start NodeID
	node  *Node
}

type fingerTable struct {
	table map[NodeID]finger
	sync.Mutex
}

// implement various methods
