package connection

import (
  "encoding/json"
  "io"
)

// AfterReadHook is a function that gets called after reading from the TCP connection.
type AfterReadHook func(*[]byte) error

// Config - Struct for containing all configuration data for the EventedConnection
type Config struct {
  Endpoint              string `json:"endpoint"`
  ConnectionTimeout     int    `json:"connectionTimeout"`
  ReadTimeout           int    `json:"readTimeout"`
  WriteTimeout          int    `json:"writeTimeout"`
  AfterReadHook         AfterReadHook
}
// Unmarshal sets config fields from the JSON data
func (conf *Config) Unmarshal(jsonBody io.Reader) (*Config, error) {
  err := json.NewDecoder(jsonBody).Decode(conf)
  return conf, err
}
