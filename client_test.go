package eventedconnection_test

import (
	. "github.com/joedursun/EventedConnection"
	"github.com/joedursun/EventedConnection/testutils"
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

// TestNewClient_Connect_Success tests that a connection can be successfully established and that
// the appropriate callbacks are called.
func TestNewClient_Connect_Success(t *testing.T) {
	done := make(chan bool)
	l, err := testutils.MockListener(done)
	if err != nil {
		t.Fatal(err)
	}
	numTimesConnected := 0 // used for counting how many attempts were made to connect to the endpoint
	numErrors := 0         // let's count how many errors were reported
	conf := Config{
		Endpoint: l.Addr().String(),
		AfterConnectHook: func() error {
			numTimesConnected++
			return nil
		},
		OnErrorHook: func(err error) error {
			numErrors++
			return nil
		},
	}
	con, err := NewClient(&conf)
	if err != nil {
		t.Error("Expected err to be nil")
	}

	err = con.Connect()
	defer con.Close()
	if err != nil {
		t.Error("Received unexpected error when connecting.")
	}
	assertEqual(t, con.IsActive(), true)
	assertEqual(t, numTimesConnected, 1)
	assertEqual(t, numErrors, 0)

	// Check to make sure that only one attempt was ever made
	err = con.Connect()
	assertEqual(t, numTimesConnected, 1)
	assertEqual(t, numErrors, 0)
	close(done)
}

// TestNewClient_Connect_Error tests that an error is returned under appropriate conditions
func TestNewClient_Connect_Fail(t *testing.T) {
	done := make(chan bool)
	numTimesConnected := 0 // used for counting how many attempts were made to connect to the endpoint
	numErrors := 0         // let's count how many errors were reported
	conf := Config{
		Endpoint: "127.0.0.1:PORT", // use obviously invalid endpoint so connection fails
		AfterConnectHook: func() error {
			numTimesConnected++
			return nil
		},
		OnErrorHook: func(err error) error {
			numErrors++
			return nil
		},
	}
	con, err := NewClient(&conf)
	if err != nil {
		t.Error("Expected err to be nil")
	}

	err = con.Connect()
	defer con.Close()
	if err == nil {
		t.Error("Expected error when connecting to invalid endpoint")
	}
	assertEqual(t, con.IsActive(), false)
	assertEqual(t, numTimesConnected, 0)
	assertEqual(t, numErrors, 1)

	// Check to make sure that only one attempt was ever made
	err = con.Connect()
	assertEqual(t, numTimesConnected, 0)
	assertEqual(t, numErrors, 1)
	close(done)
}

func assertNotNil(t *testing.T, a interface{}) {
	if a == nil {
		t.Errorf("%s == nil", a)
	}
}

func assertEqual(t *testing.T, a interface{}, b interface{}) {
	if a != b {
		t.Errorf("%s != %s", a, b)
	}
}
