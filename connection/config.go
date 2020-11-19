package connection

import (
  "encoding/json"
  "io"
  "time"
)

// AfterReadHook is a function that gets called after reading from the TCP connection.
// Returning an error from this function is a signal to close the connection. If
// instead the caller would like to know about the error but not close the connection,
// then, for example, AfterReadHook could send the error on a channel.
type AfterReadHook func([]byte) ([]byte, error)

func defaultAfterReadHook(data []byte) ([]byte, error) { return data, nil }

// Config - Struct for containing all configuration data for the EventedConnection
type Config struct {
  Endpoint              string `json:"endpoint"`

  ConnectionTimeout     time.Duration
  ReadTimeout           time.Duration
  WriteTimeout          time.Duration

  ReadBufferSize        int    `json:"readBufferSize"`
  AfterReadHook         AfterReadHook
}

// Unmarshal sets config fields from the JSON data
func (conf *Config) Unmarshal(jsonBody io.Reader) (*Config, error) {
  err := json.NewDecoder(jsonBody).Decode(conf)
  return conf, err
}

// NewConfig instantiates a config object with defaults
func NewConfig() Config {
  conf := Config{
    ReadBufferSize: 16 * 1024, // 16 KB
    AfterReadHook: defaultAfterReadHook,
    ConnectionTimeout: 30 * time.Second,
    ReadTimeout: 1 * time.Hour,
    WriteTimeout: 5 * time.Second,
  }

  return conf
}
