package testutils

import (
	"io"
	"log"
	"net"
)

// MockListener creates a TCP listener on a random port and returns
// the address of the endpoint. Useful for tests that need to connect
// to an endpoint. The listener can accept multiple connections
// and will echo any data sent to it. Use the "done" channel to
// indicate when to stop listening.
func MockListener(done chan bool) (net.Listener, error) {
	// get random available port to listen on
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}

	go func(l net.Listener) {
		defer l.Close()
		for {
			select {
			case <-done:
				return
			default:
				conn, err := l.Accept()
				if err != nil {
					log.Fatal(err)
				}

				go func(c net.Conn) {
					// Echo all incoming data.
					io.Copy(c, c)
					c.Close()
				}(conn)
			}
		}
	}(l)

	return l, nil
}
