package eventedconnection_test

import (
  "testing"
  . "github.com/joedursun/EventedConnection"
)

func TestNewClient(t *testing.T) {
  emptyConf := Config{}
  con, err := NewClient(&emptyConf)
  if con != nil {
    t.Error("Expected con to be nil")
  }

  if err == nil {
    t.Error("Expected err to be of type error but got nil")
  }
}
