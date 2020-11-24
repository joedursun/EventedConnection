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
  ReadBufferSize        int    `json:"readBufferSize"`

  ConnectionTimeout     time.Duration `json:"connectionTimeout"`
  ReadTimeout           time.Duration `json:"readTimeout"`
  WriteTimeout          time.Duration `json:"writeTimeout"`

  AfterReadHook         AfterReadHook
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
func (conf *Config) Unmarshal(jsonBody []byte) error {
  var json jsonConfig
  err := json.NewDecoder(jsonBody).Decode(&json)

  conf.Endpoint = json.Endpoint
  conf.ReadBufferSize = json.ReadBufferSize

  conf.ConnectionTimeout = time.ParseDuration(json.ConnectionTimeout)
  conf.ReadTimeout = time.ParseDuration(json.ReadTimeout)
  conf.WriteTimeout = time.ParseDuration(json.WriteTimeout)

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
