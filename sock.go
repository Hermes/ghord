package ghord

import (
	"encoding/gob"
	"net"
	"sync"
	"time"
)

type sock struct {
	host string
	conn *net.TCPConn
	used time.Time
	enc  *Encoder
	dec  *Decoder
}

func newSock(conn *net.TCPConn) *sock {
	conn.SetNoDelay(true)
	conn.SetKeepAlive(true)
	return &sock{
		host: conn.RemoteAddr().String(),
		conn: conn,
		used: time.Now(),
		enc:  NewEncoder(conn),
		dec:  NewDecoder(conn),
	}
}

// write to the sock via an encoder
func (s *sock) write(m *Message) error {
	return s.enc.Encode(m)
}

// read an encoded value from the sock
func (s *sock) read(m *Message) error {
	return s.dec.Decode(m)
}

// Create a new connection (or get it from the cache) to a node, and add it to the sock pool
// NOTE: the ...int for port is a hack to get overloading (or something like it) working
func (c *Cluster) getSock(addr string, port ...int) (*sock, error) {

	// Normiliza the address (either given full address as string, or as ip:port components)
	var address string
	if len(port) == 1 {
		address = addr + ":" + strconv.Itoa(port[0])
	} else if len(port) > 1 {
		return nil, errors.New("Malformed address")
	} else {
		address = addr
	}

	s, found := c.connCache.Access(address)
	if !found {
		conn, err := net.DialTimeout("tcp", address, c.connTimeout)
		if err != nil {
			c.error("Couldnt get tcp conn to node: %v", err)
			c.throwErr(err)
			return nil, err
		}
		return newSock(conn)
	}
	return s, nil
}

//Put the sock back on to the conn, (thread safe? ...nope lol)
func (c *Cluster) putSock(s *sock) {
	s.used = time.Now()
	c.connCache.Insert(s.host, s)
}
