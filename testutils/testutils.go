package testutils

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

// EchoServer creates a TCP listener on a random port and echoes
// any data sent through the connection. Useful for tests that need
// to connect to an endpoint. The listener can accept multiple
// connections and will echo any data sent to it. Use the "done"
// channel to indicate when to stop listening.
func EchoServer(done chan bool) (net.Listener, error) {
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

// FlakyServer creates a TCP listener on a random port and echoes
// any data sent through the connection. This server drops the connection
// after the provided duration.
func FlakyServer(done chan bool, lifetime time.Duration) (net.Listener, error) {
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
					<-time.After(lifetime) // block until time expires and then close the connection
					c.Close()
				}(conn)
			}
		}
	}(l)

	return l, nil
}

// TLSEchoServer uses the test cert and key files.
func TLSEchoServer(done chan bool, crtFilename, keyFilename string) (net.Listener, error) {
	cer, err := tls.LoadX509KeyPair(crtFilename, keyFilename)
	if err != nil {
		return nil, err
	}

	config := &tls.Config{Certificates: []tls.Certificate{cer}}
	l, err := tls.Listen("tcp", ":0", config)
	if err != nil {
		return nil, err
	}

	go func(l net.Listener) {
		defer l.Close()
		for {
			select {
			case <-done:
				fmt.Println("Done channel no longer blocking. Returning and closing connection.")
				return
			default:
				conn, err := l.Accept()
				if err != nil {
					fmt.Println(err)
					return
				}

				go func(c net.Conn) {
					io.Copy(c, c)
					c.Close()
				}(conn)
			}
		}
	}(l)

	return l, nil
}
