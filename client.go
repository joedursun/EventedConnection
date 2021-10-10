package eventedconnection

import (
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"
)

// Client gives us a stable way to connect and maintain a connection to a TCP endpoint.
// Client broadcasts 2 separate events via closing a channel: Connected and Disconnected.
// This allows any number of downstream consumers to be informed when a state change happens.
type Client struct {
	Read         chan *[]byte
	Disconnected chan struct{}
	Connected    chan struct{}

	c                 net.Conn
	connectionTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	endpoint          string
	readBufferSize    int

	afterReadHook        AfterReadHook
	afterConnectHook     AfterConnectHook
	beforeDisconnectHook BeforeDisconnectHook
	onErrorHook          OnErrorHook

	useTLS    bool
	tlsConfig *tls.Config

	closer  sync.Once
	starter sync.Once

	mutex *sync.RWMutex // allows for using this connection in multiple goroutines
}

func (conn *Client) setDefaults() {
	if conn.connectionTimeout == 0*time.Second { // default timeout for connecting
		conn.connectionTimeout = DefaultConnectionTimeout
	}

	if conn.readTimeout == 0*time.Second { // default timeout for connecting
		conn.readTimeout = DefaultReadTimeout
	}

	if conn.writeTimeout == 0*time.Second { // default timeout for connecting
		conn.writeTimeout = DefaultWriteTimeout
	}

	if conn.readBufferSize == 0 {
		conn.readBufferSize = DefaultReadBufferSize
	}

	if conn.afterReadHook == nil {
		conn.afterReadHook = defaultAfterReadHook
	}

	if conn.onErrorHook == nil {
		conn.onErrorHook = defaultOnErrorHook
	}
}

// NewClient is the Connection constructor.
func NewClient(conf *Config) (*Client, error) {
	if len(conf.Endpoint) == 0 {
		return nil, errors.New("invalid endpoint (empty string)")
	}

	conn := Client{
		endpoint:             conf.Endpoint,
		connectionTimeout:    conf.ConnectionTimeout,
		readTimeout:          conf.ReadTimeout,
		writeTimeout:         conf.WriteTimeout,
		readBufferSize:       conf.ReadBufferSize,
		afterReadHook:        conf.AfterReadHook,
		afterConnectHook:     conf.AfterConnectHook,
		beforeDisconnectHook: conf.BeforeDisconnectHook,
		onErrorHook:          conf.OnErrorHook,
		Disconnected:         make(chan struct{}),
		Connected:            make(chan struct{}),
		Read:                 make(chan *[]byte, 4), // 4 packets (up to 4 * conn.ReadBufferSize); reduces blocking when reading from connection
		mutex:                &sync.RWMutex{},
	}

	if conf.UseTLS {
		conn.tlsConfig = conf.TLSConfig
		conn.useTLS = conf.UseTLS
	}

	conn.setDefaults()

	return &conn, nil
}

// Connect attempts to establish a TCP connection to conn.Endpoint.
func (conn *Client) Connect() error {
	var err error
	var connection net.Conn

	conn.starter.Do(func() {
		if conn.useTLS {
			connection, err = tls.Dial("tcp", conn.endpoint, conn.tlsConfig)
		} else {
			connection, err = net.DialTimeout("tcp", conn.endpoint, conn.connectionTimeout)
		}

		if err != nil {
			conn.onErrorHook(err)
			return // return early so we don't execute other hooks, send Connected event, etc.
		}

		conn.setConnection(connection)
		defer conn.afterConnect()

		go conn.readFromConn()
		close(conn.Connected) // broadcast that TCP connection to interface was established
	})
	return err
}

func (conn *Client) Reconnect() error {
	conn.Close()
	conn.reset()
	return conn.Connect()
}

func (conn *Client) reset() {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	conn.Disconnected = make(chan struct{})
	conn.Connected = make(chan struct{})
	conn.starter = sync.Once{}
	conn.closer = sync.Once{}
}

func (conn *Client) setConnection(c net.Conn) {
	conn.mutex.Lock()
	conn.c = c
	conn.mutex.Unlock()
}

func (conn *Client) afterConnect() {
	if conn.afterConnectHook != nil {
		err := conn.afterConnectHook()
		if err != nil {
			conn.onErrorHook(err)
		}
	}
}

// IsActive provides a way to check if the connection is still usable
func (conn *Client) IsActive() bool {
	conn.mutex.RLock()
	defer conn.mutex.RUnlock()

	return conn.c != nil
}

// Write provides a thread-safe way to send messages to the endpoint. If the connection is
// nil (e.g. closed) then this is a noop.
func (conn *Client) Write(data *[]byte) error {
	var err error

	connection := conn.rawConnection()
	if connection == nil {
		err = errors.New("called Write with nil connection")
		conn.onErrorHook(err)
		return err
	}

	err = connection.SetWriteDeadline(time.Now().Add(conn.GetWriteTimeout()))
	if err != nil {
		conn.onErrorHook(err)
		defer conn.Close()
		return err
	}

	_, err = connection.Write(*data)
	if err != nil {
		conn.onErrorHook(err)
		defer conn.Close()
	}

	return err
}

// Close closes the TCP connection. Broadcasts via the Disconnected channel.
// Safe to call more than once, however will only close an open TCP connection on the first call.
// Closes the conn.Disconnected chan prior to closing the TCP connection to allow
// short-circuiting of downstream `select` blocks and avoid attempts to write to it
// by the caller.
func (conn *Client) Close() {
	conn.mutex.Lock()
	defer conn.mutex.Unlock()

	conn.closer.Do(func() {
		if conn.beforeDisconnectHook != nil {
			if err := conn.beforeDisconnectHook(); err != nil {
				conn.onErrorHook(err)
			}
		}

		close(conn.Disconnected) // broadcast that TCP connection to interface was closed
		if conn.c != nil {
			conn.c.Close()
			conn.c = nil // set C to nil so it's clear the connection cannot be used
		}
	})
}

// Disconnect is an alias for conn.Close()
func (conn *Client) Disconnect() {
	conn.Close()
}

// processResponse handles data coming from the TCP connection
// and sends it through the conn.Read chan
func (conn *Client) processResponse(data []byte) (err error) {
	var processed []byte

	if len(data) > 0 {
		processed, err = conn.afterReadHook(data)
		if err != nil {
			conn.onErrorHook(err)
		}
		conn.Read <- &processed
	}

	return err
}

// readFromConn reads data from the connection into a buffer and then
// passes onto processResponse. In the event of an error the connection
// is closed.
func (conn *Client) readFromConn() error {
	defer conn.Close()

	buffer := make([]byte, conn.GetReadBufferSize())
	for {
		var err error
		connection := conn.rawConnection()

		if connection == nil {
			err = errors.New("unable to read from nil connection")
			conn.onErrorHook(err)
			return err
		}

		err = connection.SetReadDeadline(time.Now().Add(conn.GetReadTimeout()))
		if err != nil {
			conn.onErrorHook(err)
			return err
		}

		numBytesRead, err := connection.Read(buffer)
		if numBytesRead > 0 {
			res := make([]byte, numBytesRead)
			// Copy the buffer so it's safe to pass along
			copy(res, buffer[:numBytesRead])
			err = conn.processResponse(res)
		}

		if err != nil {
			conn.onErrorHook(err)
			return err
		}
	}
}

// rawConnection is used for getting the underlying TCP connection
// in a thread safe way
func (conn *Client) rawConnection() net.Conn {
	conn.mutex.RLock()
	defer conn.mutex.RUnlock()
	return conn.c
}

// GetEndpoint returns the value of conn.endpoint
func (conn *Client) GetEndpoint() string {
	return conn.endpoint
}

// GetReadBufferSize returns the value of conn.readBufferSize
func (conn *Client) GetReadBufferSize() int {
	return conn.readBufferSize
}

// GetWriteTimeout returns the value of conn.writeTimeout
func (conn *Client) GetWriteTimeout() time.Duration {
	return conn.writeTimeout
}

// GetReadTimeout returns the value of conn.readTimeout
func (conn *Client) GetReadTimeout() time.Duration {
	return conn.readTimeout
}

// GetConnectionTimeout returns the value of conn.connectionTimeout
func (conn *Client) GetConnectionTimeout() time.Duration {
	return conn.connectionTimeout
}
