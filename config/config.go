package config

import (
  "encoding/json"
  "io"
)

// Endpoint - Configuration for generic endpoints
type Endpoint struct {
  Host string `json:"host"`
}

// Config - Struct for containing all configuration data for the StableConnection
type Config struct {
  Endpoint  Endpoint `json:"endpoint"`
  ReadTimeout     int    `json:"readTimeout"`
  WriteTimeout    int    `json:"writeTimeout"`
}
// Unmarshal sets config fields from the JSON data
func (conf *Config) Unmarshal(jsonBody io.Reader) (*Config, error) {
  err := json.NewDecoder(jsonBody).Decode(conf)
  return conf, err
}