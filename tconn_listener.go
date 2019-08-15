package turbo

import (
	"errors"
	"net"
	"time"
)

var CONN_ERROR error = errors.New("STOP LISTENING")

//远程的listener
type StoppedListener struct {
	*net.TCPListener
	stop      chan bool
	conn      chan *net.TCPConn
	keepalive time.Duration
}

//accept
func (self *StoppedListener) Accept() (*net.TCPConn, error) {
	for {
		var conn *net.TCPConn
		var err error
		go func() {
			c, e := self.AcceptTCP()
			if e != nil {
				err = e
			}
			self.conn <- c
		}()
		select {
		case <-self.stop:
			return nil, CONN_ERROR
		case conn = <-self.conn:
			//do nothing
		}

		if nil == err {
			conn.SetKeepAlive(true)
			conn.SetKeepAlivePeriod(self.keepalive)
		} else {
			return nil, err
		}

		return conn, err
	}
}
