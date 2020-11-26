package connection

import (
  "encoding/json"
  "io"
  "log"
  "os"
  "time"
)

// AfterReadHook is a function that gets called after reading from the TCP connection.
// Returning an error from this function is a signal to close the connection. If
// instead the caller would like to know about the error but not close the connection,
// then, for example, AfterReadHook could send the error on a channel.
type AfterReadHook func([]byte) ([]byte, error)

// AfterConnectHook is called just after a connection is established.
type AfterConnectHook func() error

// BeforeDisconnectHook is called just before a connection is terminated.
// This hook is only called before a termination originating on this end of
// the connection (ie. if EventedConnection.Endpoint closes the connection
// or a timeout occurs then this hook is not called). Use the OnError callback
// to handle those cases.
type BeforeDisconnectHook func() error

// OnErrorHook will be called whenever an error occurs within the scope of an EventedConnection
// method. Useful for logging or event notifications for example.
type OnErrorHook func(error) error

func defaultAfterReadHook(data []byte) ([]byte, error) { return data, nil }
func defaultAfterConnectHook() error { return nil }
func defaultBeforeDisconnectHook() error { return nil }
func defaultOnErrorHook(err error) error { return err }

// Config - Struct for containing all configuration data for the EventedConnection
type Config struct {
  Endpoint              string `json:"endpoint"`
  ReadBufferSize        int    `json:"readBufferSize"`

  ConnectionTimeout     time.Duration `json:"connectionTimeout"`
  ReadTimeout           time.Duration `json:"readTimeout"`
  WriteTimeout          time.Duration `json:"writeTimeout"`

  AfterReadHook         AfterReadHook
  AfterConnectHook      AfterConnectHook
  BeforeDisconnectHook  BeforeDisconnectHook
  OnErrorHook           OnErrorHook
}

// jsonConfig is used as a temp struct to unmarshal JSON into in order to properly parse
// the duration attributes
type jsonConfig struct {
  Endpoint              string `json:"endpoint"`
  ConnectionTimeout     string `json:"connectionTimeout"`
  ReadTimeout           string `json:"readTimeout"`
  WriteTimeout          string `json:"writeTimeout"`

  ReadBufferSize        int    `json:"readBufferSize"`
}

// Unmarshal sets config fields from the JSON data. The timeout fields
// are expected to conform to strings parsable by time.ParseDuration
func (conf *Config) Unmarshal(jsonBody io.Reader) error {
  var jc jsonConfig
  err := json.NewDecoder(jsonBody).Decode(&jc)
  if err != nil { return err }

  conf.Endpoint = jc.Endpoint
  conf.ReadBufferSize = jc.ReadBufferSize

  conf.ConnectionTimeout, err = time.ParseDuration(jc.ConnectionTimeout)
  if err != nil { return err }

  conf.ReadTimeout, err = time.ParseDuration(jc.ReadTimeout)
  if err != nil { return err }

  conf.WriteTimeout, err = time.ParseDuration(jc.WriteTimeout)

  return err
}

// NewConfig instantiates a config object with defaults
func NewConfig() Config {
  l := log.New(os.Stderr, "", 0)

  conf := Config{
    ReadBufferSize: 16 * 1024, // 16 KB
    ConnectionTimeout: 30 * time.Second,
    ReadTimeout: 1 * time.Hour,
    WriteTimeout: 5 * time.Second,

    // Write to stderr by default
    OnErrorHook: func(err error) error {
      l.Println(err)
      return err
    },
  }

  return conf
}
