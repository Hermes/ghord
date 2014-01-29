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
	return s.dec.Decode(m).(*Message)
}
