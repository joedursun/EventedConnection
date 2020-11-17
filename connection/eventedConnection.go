package connection

import (
  "errors"
  "net"
  "sync"
  "time"
)

// TCPReadTimeout sets the amount of time to wait for a packet from the endpoint before considering the connection dead
const TCPReadTimeout = 10 * time.Minute
const readBufferSize = 16 * 1024 // 16 KB

// EventedConnection gives us a stable way to connect and maintain a connection to a TCP endpoint.
// EventedConnection broadcasts 3 separate events via closing a channel: Connected, Disconnected, and Canceled.
// This allows any number of downstream consumers to be informed when a state change happens.
type EventedConnection struct {
  C                    net.Conn
  ConnectionTimeout    int
  Endpoint             string
  Read                 chan []byte
  Disconnected         chan struct{}
  Connected            chan struct{}
  Canceled             chan struct{}

  writeChan            chan []byte
  mutex                *sync.RWMutex // allows for using this connection in multiple goroutines
  active               bool
}

// NewEventedConnection is the Connection constructor.
func NewEventedConnection(conf *Config, endpoint string) (*EventedConnection, error) {
  conn := EventedConnection{}
  if len(endpoint) == 0 {
    return nil, errors.New("Invalid endpoint (empty string)")
  }

  conn.Endpoint = endpoint
  conn.ConnectionTimeout = 5 // default timeout for connecting
  if conf.Timeout > 0 {
    conn.ConnectionTimeout = conf.Timeout
  }

  conn.Disconnected = make(chan struct{}, 0)
  conn.Connected = make(chan struct{}, 0)
  conn.Canceled = make(chan struct{}, 0)
  conn.Read = make(chan []byte, 5) // buffer of 5 packets (up to 5 * readBufferSize). reduces blocking when reading from connection
  conn.writeChan = make(chan []byte)
  conn.mutex = &sync.RWMutex{}
  conn.active = false
  return &conn, nil
}

// Connect attempts to establish a TCP connection to conn.Endpoint.
func (conn *EventedConnection) Connect() error {
  timeout := time.Duration(conn.ConnectionTimeout) // must cast int to Duration if the int is not a constant
  tcpConn, err := net.DialTimeout("tcp", conn.Endpoint, timeout*time.Second)
  if err != nil {
    conn.Cancel()
  } else {
    conn.mutex.Lock()
    conn.C = tcpConn
    conn.active = true
    conn.mutex.Unlock()
    go conn.writeToConn()
    go conn.readFromConn()
    close(conn.Connected) // broadcast that TCP connection to interface was established
    return
  }
}

// Write provides a thread-safe way to send messages to the endpoint. If the connection is
// nil (e.g. closed) then this is a noop.
func (conn *EventedConnection) Write(data []byte) error {
  conn.mutex.RLock() // obtain lock before checking if connection is dead so value isn't changed while reading
  defer conn.mutex.RUnlock()
  if conn.C == nil {
    // silently drop packet to prevent pooling of data in calling goroutines.
    // if we don't do this then we'd either block (pausing execution until conn.writeChan is read)
    // or, in the case where caller is in separate goroutine, then the caller could accumulate goroutines
    // waiting to write to the TCP connection.
    return errors.New("connection is nil and data was not sent")
  }

  if conn.active {
    conn.writeChan <- data
  } else {
    return errors.New("connection is not active and data was not sent")
  }

  return nil
}

// writeToConn receives messages on writeChan and writes them to the TCP connection. If any error occurs
// the connection is closed and this function returns. In the event of an intentional disconnect
// event this function also returns.
func (conn *EventedConnection) writeToConn() {
  defer conn.Close()

  for {
    select {
    case data := <-conn.writeChan:
      err := conn.C.SetWriteDeadline(time.Now().Add(5 * time.Second))
      if err != nil {
        return
      }
      _, err = conn.C.Write(data)
      if err != nil {
        return
      }
    case <-conn.Disconnected:
      return
    case <-conn.Canceled:
      return
    }
  }
}

// Cancel aborts the connection process
func (conn *EventedConnection) Cancel() {
  close(conn.Canceled) // broadcast that TCP connection to interface was established
}

// Close closes the TCP connection. Broadcasts via the Canceled and Disconnected channels.
// Provides a 3 second grace period after the Canceled event for consumers to prepare for the
// disconnect.
func (conn *EventedConnection) Close() {
  conn.mutex.Lock()
  conn.Cancel()
  time.Sleep(3 * time.Second) // grace period before closing the connection
  conn.active = false         // set "active" flag to false so we no longer queue up packets to send
  if conn.C != nil {
    conn.C.Close()
    close(conn.Disconnected) // broadcast that TCP connection to interface was closed
  }
  conn.mutex.Unlock()
}

// Disconnect is an alias for conn.Close()
func (conn *EventedConnection) Disconnect() {
  conn.Close()
}

// processResponse handles data coming from TCP connection
// and sends it through the conn.Read chan
func (conn *EventedConnection) processResponse(data) {
  if len(data) > 0 {
    conn.Read <- data
  }
}

func (conn *EventedConnection) readFromConn() error {
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
