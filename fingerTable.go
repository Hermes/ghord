package ghord

import (
	"sync"
)

type finger struct {
	start NodeID
	node  *Node
}

type fingerTable struct {
	table []finger
	sync.Mutex
}

// implement various methods
