package connection

import (
  "errors"
  "net"
  "sync"
  "time"
)

// TCPReadTimeout sets the amount of time to wait for a record from the endpoint before considering the connection dead
const TCPReadTimeout = 10 * time.Minute
const readBufferSize = 16 * 1024 // 16 KB

// StableConnection gives us a stable way to connect and maintain a connection to a TCP endpoint.
type StableConnection struct {
  C                    net.Conn
  ConnectionTimeout    int
  Endpoint             string
  Read                 chan []byte
  writeChan            chan []byte
  active               bool
}

// NewStableConnection is the Connection constructor.
func NewStableConnection(conf *Config, endpoint string) (*StableConnection, error) {
  conn := StableConnection{}
  if len(endpoint) == 0 {
    return nil, errors.New("Invalid endpoint (empty string)")
  }
  conn.Endpoint = endpoint
  conn.ConnectionTimeout = 5 // default timeout for connecting
  if conf.Timeout > 0 {
    conn.ConnectionTimeout = conf.Timeout
  }
  conn.Read = make(chan []byte, 5) // buffer of 5 packets (up to 5 * readBufferSize). reduces blocking when reading from connection
  conn.active = false
  return &conn, nil
}

// Connect attempts to establish a TCP connection to Connection.Endpoint.
func (conn *StableConnection) Connect() error {
  timeout := time.Duration(conn.ConnectionTimeout) // must cast int to Duration if the int is not a constant
  tcpConn, err := net.DialTimeout("tcp", conn.Endpoint, timeout*time.Second)
  if err != nil {
    return err
  }
  conn.C = tcpConn
  conn.active = true
  go conn.readFromConn()
  return nil
}

// Write provides a thread-safe way to send messages to the endpoint.
func (conn *StableConnection) Write(record []byte) {
  // TODO write a consumer for this chan
  if conn.active {
    conn.writeChan <- record
  }
}

// Close closes the TCP connection
func (conn *StableConnection) Close() {
  if conn.C != nil {
    conn.C.Close()
  }
}

// processResponse handles data coming from TCP connection
// and sends it through the conn.Read chan
func (conn *StableConnection) processResponse(data) {
  if len(data) > 0 {
    conn.Read <- data
  }
}

func (conn *StableConnection) readFromConn() error {
  defer conn.Close()

  buffer := make([]byte, readBufferSize)
  for {
    if conn.C == nil {
      return errors.New("unable to read from nil connection")
    }

    err := conn.C.SetReadDeadline(time.Now().Add(TCPReadTimeout))
    if err != nil {
      return err
    }

    numBytesRead, err := conn.C.Read(buffer)
    if numBytesRead > 0 {
      res := make([]byte, numBytesRead)
      // Copy the buffer so it's safe to pass along
      copy(res, buffer[:numBytesRead])
      conn.processResponse(res)
    }

    if err != nil {
      return
    }
  }
}
