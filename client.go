package connection

import (
  "crypto/tls"
  "errors"
  "net"
  "sync"
  "time"
)

// TCPReadTimeout sets the amount of time to wait for a packet from the endpoint before considering the connection dead
const TCPReadTimeout = 1 * time.Hour

// Client gives us a stable way to connect and maintain a connection to a TCP endpoint.
// Client broadcasts 3 separate events via closing a channel: Connected and Disconnected.
// This allows any number of downstream consumers to be informed when a state change happens.
type Client struct {
  C                    net.Conn
  ConnectionTimeout    time.Duration
  ReadTimeout          time.Duration
  WriteTimeout         time.Duration
  Endpoint             string
  ReadBufferSize       int
  Read                 chan *[]byte
  Disconnected         chan struct{}
  Connected            chan struct{}

  afterReadHook        AfterReadHook
  afterConnectHook     AfterConnectHook
  beforeDisconnectHook BeforeDisconnectHook
  onErrorHook          OnErrorHook

  useTLS               bool
  tlsConfig            *tls.Config

  closer               sync.Once
  starter              sync.Once

  writeChan            chan *[]byte
  mutex                *sync.RWMutex // allows for using this connection in multiple goroutines
  active               bool
}

// NewClient is the Connection constructor.
func NewClient(conf *Config) (*Client, error) {
  if len(conf.Endpoint) == 0 {
    return nil, errors.New("Invalid endpoint (empty string)")
  }

  conn := Client{}
  conn.Endpoint = conf.Endpoint
  if conf.ConnectionTimeout == 0 * time.Second { // default timeout for connecting
    conn.ConnectionTimeout = 30 * time.Second
  }

  conn.ReadTimeout = conf.ReadTimeout
  if conf.ReadTimeout == 0 * time.Second { // default timeout for receiving data
    conn.ReadTimeout = 1 * time.Hour
  }

  conn.WriteTimeout = conf.WriteTimeout
  if conf.WriteTimeout == 0 * time.Second { // default timeout for sending data
    conn.WriteTimeout = 5 * time.Second
  }

  conn.ReadBufferSize = conf.ReadBufferSize
  if conf.ReadBufferSize == 0 {
    conn.ReadBufferSize = 16 * 1024 // 16 KB
  }

  if conf.UseTLS {
    conn.tlsConfig = &conf.TLSConfig
    conn.useTLS = conf.UseTLS
  }

  conn.Disconnected = make(chan struct{}, 0)
  conn.Connected = make(chan struct{}, 0)
  conn.Read = make(chan *[]byte, 4) // buffer of 4 packets (up to 4 * conn.ReadBufferSize). reduces blocking when reading from connection
  conn.writeChan = make(chan *[]byte) // not buffered so that the Write method can block and the caller will know if the write was successful or not
  conn.mutex = &sync.RWMutex{}
  conn.active = false

  conn.afterReadHook = conf.AfterReadHook
  conn.afterConnectHook = conf.AfterConnectHook
  conn.beforeDisconnectHook = conf.BeforeDisconnectHook
  conn.onErrorHook = conf.OnErrorHook

  return &conn, nil
}

// Connect attempts to establish a TCP connection to conn.Endpoint.
func (conn *Client) Connect() error {
  var err error
  var connection net.Conn

  conn.starter.Do(func() {
    if conn.useTLS {
      connection, err = tls.Dial("tcp", conn.Endpoint, conn.tlsConfig)
    } else {
      connection, err = net.DialTimeout("tcp", conn.Endpoint, conn.ConnectionTimeout)
    }

    if err != nil {
      if conn.onErrorHook != nil { conn.onErrorHook(err) }
      return // return early so we don't execute other hooks, send Connected event, etc.
    }

    conn.mutex.Lock()
    conn.C = connection
    conn.active = true
    conn.mutex.Unlock()

    if conn.afterConnectHook != nil {
      err = conn.afterConnectHook()
      if err != nil && conn.onErrorHook != nil { conn.onErrorHook(err) }
    }

    go conn.writeToConn()
    go conn.readFromConn()
    close(conn.Connected) // broadcast that TCP connection to interface was established
    return
  })
  return err
}

// Write provides a thread-safe way to send messages to the endpoint. If the connection is
// nil (e.g. closed) then this is a noop.
func (conn *Client) Write(data *[]byte) error {
  var err error

  conn.mutex.RLock() // obtain lock before checking if connection is dead so value isn't changed while reading
  defer conn.mutex.RUnlock()
  if conn.C == nil {
    err = errors.New("called Write with nil connection")
    if conn.onErrorHook != nil { conn.onErrorHook(err) }
    return err
  }

  if conn.active {
    conn.writeChan <- data
  } else {
    err = errors.New("connection is not active and data was not sent")
    if conn.onErrorHook != nil { conn.onErrorHook(err) }
  }

  return err
}

// writeToConn receives messages on writeChan and writes them to the TCP connection. If any error occurs
// the connection is closed and this function returns. In the event of an intentional disconnect
// event this function also returns.
func (conn *Client) writeToConn() {
  defer conn.Close()

  for {
    select {
    case data := <-conn.writeChan:
      err := conn.C.SetWriteDeadline(time.Now().Add(conn.WriteTimeout))
      if err != nil {
        if conn.onErrorHook != nil { conn.onErrorHook(err) }
        return
      }

      // Obtain lock so that conn.C is not closed while attempting to write
      conn.mutex.RLock()
      _, err = conn.C.Write(*data)
      conn.mutex.RUnlock()

      if err != nil {
        if conn.onErrorHook != nil { conn.onErrorHook(err) }
        return
      }
    case <-conn.Disconnected:
      return
    }
  }
}

// Close closes the TCP connection. Broadcasts via the Disconnected channel.
// Safe to call more than once, however will only close an open TCP connection on the first call.
// Closes the conn.Disconnected chan prior to closing the TCP connection to allow
// short-circuiting of downstream `select` blocks and avoid attempts to write to it
// by the caller.
func (conn *Client) Close() {
  conn.closer.Do(func() {
    conn.mutex.Lock()
    conn.active = false         // set "active" flag to false so we no longer queue up packets to send

    if conn.beforeDisconnectHook != nil {
      err := conn.beforeDisconnectHook()
      if err != nil && conn.onErrorHook != nil { conn.onErrorHook(err) }
    }

    close(conn.Disconnected)    // broadcast that TCP connection to interface was closed
    if conn.C != nil {
      conn.C.Close()
      conn.C = nil              // set C to nil so it's clear the connection cannot be used
    }

    conn.mutex.Unlock()
  })
}

// Disconnect is an alias for conn.Close()
func (conn *Client) Disconnect() {
  conn.Close()
}

// processResponse handles data coming from TCP connection
// and sends it through the conn.Read chan
func (conn *Client) processResponse(data []byte) error {
  if len(data) > 0 {
    processed, err := conn.afterReadHook(data)
    if err != nil {
      if conn.onErrorHook != nil { conn.onErrorHook(err) }
      return err
    }
    conn.Read <- &processed
  }

  return nil
}

// readFromConn reads data from the connection into a buffer and then
// passes onto processResponse. In the event of an error the connection
// is closed.
func (conn *Client) readFromConn() error {
  defer conn.Close()

  buffer := make([]byte, conn.ReadBufferSize)
  for {
    var err error

    if conn.C == nil {
      err = errors.New("unable to read from nil connection")
      if conn.onErrorHook != nil { conn.onErrorHook(err) }
      return err
    }

    err = conn.C.SetReadDeadline(time.Now().Add(conn.ReadTimeout))
    if err != nil {
      if conn.onErrorHook != nil { conn.onErrorHook(err) }
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
      if conn.onErrorHook != nil { conn.onErrorHook(err) }
      return err
    }
  }
}