package eventedconnection_test

import (
	. "github.com/joedursun/EventedConnection"
	"testing"
	"time"
)

func TestNewClient_ConfigDefaults(t *testing.T) {
	emptyConf := Config{}
	con, err := NewClient(&emptyConf)
	if con != nil {
		t.Error("Expected con to be nil")
	}

	if err == nil {
		t.Error("Expected err to be of type error but got nil")
	}

	conf := Config{Endpoint: "localhost:5555"}
	con, err = NewClient(&conf)
	assertEqual(t, con.Endpoint, conf.Endpoint)
	assertEqual(t, con.ConnectionTimeout, 30*time.Second)
	assertEqual(t, con.ReadTimeout, 1*time.Hour)
	assertEqual(t, con.WriteTimeout, 5*time.Second)
	assertEqual(t, con.ReadBufferSize, 16*1024)
}

func assertEqual(t *testing.T, a interface{}, b interface{}) {
	if a != b {
		t.Fatalf("%s != %s", a, b)
	}
}
