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

  afterReadHook        AfterReadHook

  canceledSent         sync.Once
  closer               sync.Once
  starter              sync.Once

  writeChan            chan []byte
  mutex                *sync.RWMutex // allows for using this connection in multiple goroutines
  active               bool
}

// NewEventedConnection is the Connection constructor.
func NewEventedConnection(conf *Config) (*EventedConnection, error) {
  if len(conf.Endpoint) == 0 {
    return nil, errors.New("Invalid endpoint (empty string)")
  }

  conn := EventedConnection{}
  conn.Endpoint = conf.Endpoint
  conn.ConnectionTimeout = 5 // default timeout for connecting
  if conf.ConnectionTimeout > 0 {
    conn.ConnectionTimeout = conf.ConnectionTimeout
  } else {
    conn.ConnectionTimeout = 30
  }

  conn.Disconnected = make(chan struct{}, 0)
  conn.Connected = make(chan struct{}, 0)
  conn.Canceled = make(chan struct{}, 0)
  conn.Read = make(chan []byte, 4) // buffer of 4 packets (up to 4 * readBufferSize). reduces blocking when reading from connection
  conn.writeChan = make(chan []byte) // not buffered so that the Write method can block and the caller will know if the write was successful or not
  conn.mutex = &sync.RWMutex{}
  conn.active = false

  if conf.AfterReadHook != nil {
    conn.afterReadHook = conf.AfterReadHook
  } else {
    conn.afterReadHook = func(data []byte) ([]byte, error) { return data, nil }
  }

  return &conn, nil
}

// Connect attempts to establish a TCP connection to conn.Endpoint.
func (conn *EventedConnection) Connect() {
  conn.starter.Do(func() {
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
  })
}

// Write provides a thread-safe way to send messages to the endpoint. If the connection is
// nil (e.g. closed) then this is a noop.
func (conn *EventedConnection) Write(data []byte) error {
  conn.mutex.RLock() // obtain lock before checking if connection is dead so value isn't changed while reading
  defer conn.mutex.RUnlock()
  if conn.C == nil {
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

      // Obtain lock so that conn.C is not closed while attempting to write
      conn.mutex.RLock()
      _, err = conn.C.Write(data)
      conn.mutex.RUnlock()

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

// Cancel aborts the connection process and is safe to call more than once.
// Subsequent calls will have no effect. This method is called if the attempt
// to establish a connection fails and warns any downstream caller or event
// listener that a connection was aborted.
func (conn *EventedConnection) Cancel() {
  conn.canceledSent.Do(func() {
    close(conn.Canceled) // broadcast that TCP connection to interface was canceled
  })
}

// Close closes the TCP connection. Broadcasts via the Canceled and Disconnected channels.
// Safe to call more than once, however will only close an open TCP connection on the first call.
func (conn *EventedConnection) Close() {
  conn.closer.Do(func() {
    conn.mutex.Lock()
    conn.Cancel()
    conn.active = false         // set "active" flag to false so we no longer queue up packets to send

    if conn.C != nil {
      conn.C.Close()
    }

    close(conn.Disconnected) // broadcast that TCP connection to interface was closed
    close(conn.Read)         // close the read chan too so any read loops are aware
    conn.mutex.Unlock()
  })
}

// Disconnect is an alias for conn.Close()
func (conn *EventedConnection) Disconnect() {
  conn.Close()
}

// processResponse handles data coming from TCP connection
// and sends it through the conn.Read chan
func (conn *EventedConnection) processResponse(data []byte) error {
  if len(data) > 0 {
    data, err := conn.afterReadHook(data)
    if err != nil {
      return err
    }
    conn.Read <- data
  }

  return nil
}

// readFromConn reads data from the connection into a buffer and then
// passes onto processResponse. In the event of an error the connection
// is closed.
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
      err = conn.processResponse(res)
    }

    if err != nil {
      return err
    }
  }
}
