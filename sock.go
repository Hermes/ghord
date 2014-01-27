package ghord

import (
	"encoding/gob"
	"net"
	"sync"
	"time"
)

// A cache of sockets to various nodes, thread safe
type sockPool struct {
	pool map[string]*sock
	sync.Mutex
}

func newSockPool() sockPool {
	pool := sockPool{}
	pool.pool = make(map[string]*sock)
	return pool
}

func (s sockPool) getSock(host string) *sock {
	s.Lock()
	defer s.Unlock()
	sock := s.pool[host]
	delete(s.pool, host)
	return sock
}

func (s sockPool) putSock(host string, sock *sock) {
	s.Lock()
	defer s.Unlock()
	s.pool[host] = sock
}

type sock struct {
	host string
	conn *net.TCPConn
	used time.Time
	enc  *gob.Encoder
	dec  *gob.Decoder
}

func newSock(conn *net.TCPConn) *sock {
	conn.SetNoDelay(true)
	conn.SetKeepAlive(true)
	return &sock{
		host: conn.RemoteAddr().String(),
		conn: conn,
		used: time.Now(),
		enc:  gob.NewEncoder(conn),
		dec:  gob.NewDecoder(conn),
	}
}

func (s *sock) write(m *Message) error {
	return s.enc.Encode(m)
}

func (s *sock) read(m *Message) error {
	return s.dec.Decode(m)
}
