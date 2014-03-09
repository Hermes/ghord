package ghord

import (
	"bytes"
	"math"
	"math/rand"
)

// calculate offset, id + 2^(exp-1) mod 2^m
func powerOffset(id []byte, exp int) int {

}

// Calculate a random number between n, and m
func randRange(min, max int64) int64 {
	return rand.Int63n(max-min) + min
}

// Checks if key is between id1 and id2 exclusivly
func between(id1, id2, key NodeID) bool {
	return bytes.Compare(key, id1) == 1 &&
		bytes.Compare(key, id2) == -1
}

// Checks if key E (id1, id2]
func betweenRightInc(id1, id2, key NodeID) bool {
	return (bytes.Compare(key, id1) == 1 &&
		bytes.Compare(key, id2) == -1) ||
		bytes.Equal(key, id2)
}

// Checks if key E [id1, id2)
func betweenLeftInc(id1, id2, key NodeID) bool {
	return (bytes.Compare(key, id1) == 1 &&
		bytes.Compare(key, id2) == -1) ||
		bytes.Equal(key, id1)
}

// Cluster logging //

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
