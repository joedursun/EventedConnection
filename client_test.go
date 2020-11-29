package eventedconnection_test

import (
	. "github.com/joedursun/EventedConnection"
	"testing"
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
