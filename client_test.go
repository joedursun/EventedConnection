package eventedconnection_test

import (
	. "github.com/joedursun/EventedConnection"
	"testing"
	"time"
)

func TestNewClient_Config(t *testing.T) {
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

	if err != nil {
		t.Error("Expected err to be nil")
	}

	assertEqual(t, con.GetEndpoint(), conf.Endpoint)
	assertEqual(t, con.GetConnectionTimeout(), 30*time.Second)
	assertEqual(t, con.GetReadTimeout(), 1*time.Hour)
	assertEqual(t, con.GetWriteTimeout(), 5*time.Second)
	assertEqual(t, con.GetReadBufferSize(), 16*1024)

	conf = Config{
		Endpoint:          "localhost:5555",
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      4 * time.Second,
		ConnectionTimeout: 8 * time.Second,
		ReadBufferSize:    2 * 1024,
	}

	con, err = NewClient(&conf)
	if err != nil {
		t.Error("Expected err to be nil")
	}

	assertEqual(t, con.GetEndpoint(), conf.Endpoint)
	assertEqual(t, con.GetConnectionTimeout(), conf.ConnectionTimeout)
	assertEqual(t, con.GetReadTimeout(), conf.ReadTimeout)
	assertEqual(t, con.GetWriteTimeout(), conf.WriteTimeout)
	assertEqual(t, con.GetReadBufferSize(), 2*1024)
}

func assertEqual(t *testing.T, a interface{}, b interface{}) {
	if a != b {
		t.Errorf("%s != %s", a, b)
	}
}
