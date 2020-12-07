package eventedconnection_test

import (
	"crypto/tls"
	. "github.com/joedursun/EventedConnection"
	"github.com/joedursun/EventedConnection/testutils"
	"math/rand"
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

func TestNewClient_ConfigTLS(t *testing.T) {
	done := make(chan bool)
	l, err := testutils.TLSEchoServer(done, "./testutils/testserver.crt", "./testutils/testserver.key")
	if err != nil {
		t.Fatal(err)
	}

	numTimesConnected := 0 // used for counting how many attempts were made to connect to the endpoint
	numErrors := 0         // let's count how many errors were reported
	TLSConf := &tls.Config{InsecureSkipVerify: true}
	conf := Config{
		Endpoint:    l.Addr().String(),
		ReadTimeout: 1 * time.Second,
		UseTLS:      true,
		TLSConfig:   TLSConf,
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
		t.Error(err)
	}
	assertEqual(t, con.IsActive(), true)
	assertEqual(t, numTimesConnected, 1)

	// Call connect again and check if a second attempt to connect is made
	err = con.Connect()
	if err != nil {
		t.Error(err)
	}
	assertEqual(t, numTimesConnected, 1)

	payload := []byte("Testing TLS payload")
	con.Write(&payload)
	select {
	case received := <-con.Read:
		if string(*received) != string(payload) {
			t.Errorf("Expected %s; received %s", received, payload)
		}
	case <-time.After(3 * time.Second):
		t.Error("Read deadline passed and test timed out")
	}
	close(done)
}

// TestNewClient_Connect_Success tests that a connection can be successfully established and that
// the appropriate callbacks are called.
func TestClient_Connect_Success(t *testing.T) {
	done := make(chan bool)
	l, err := testutils.EchoServer(done)
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
	_ = con.Connect()
	assertEqual(t, numTimesConnected, 1)
	assertEqual(t, numErrors, 0)
	close(done)
}

// TestNewClient_Connect_Error tests that an error is returned under appropriate conditions
func TestClient_Connect_Fail(t *testing.T) {
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
	_ = con.Connect()
	assertEqual(t, numTimesConnected, 0)
	assertEqual(t, numErrors, 1)
	close(done)
}

func TestClient_Close(t *testing.T) {
	done := make(chan bool)
	l, err := testutils.EchoServer(done)
	if err != nil {
		t.Error(err)
	}

	calledDisconnectHook := false
	conf := Config{
		Endpoint: l.Addr().String(),
		BeforeDisconnectHook: func() error {
			calledDisconnectHook = true
			return nil
		},
	}

	con, err := NewClient(&conf)
	if err != nil {
		t.Error("Expected err to be nil")
	}

	err = con.Connect()
	if err != nil {
		t.Error("Received error when connecting.")
	}

	assertEqual(t, con.IsActive(), true)
	payload := []byte("test")
	err = con.Write(&payload)
	assertEqual(t, err, nil)
	con.Close()
	assertEqual(t, con.IsActive(), false)
	assertEqual(t, calledDisconnectHook, true)

	err = con.Write(&payload)
	assertNotNil(t, err)
	con.Close() // call again to test if it panics

	close(done)
}

func TestClient_ReadWrite(t *testing.T) {
	done := make(chan bool)
	l, err := testutils.EchoServer(done)
	if err != nil {
		t.Error(err)
	}

	conf := Config{
		Endpoint: l.Addr().String(),
		AfterReadHook: func(data []byte) ([]byte, error) {
			processed := append(data, '!')
			return processed, nil
		},
	}

	con, err := NewClient(&conf)
	if err != nil {
		t.Error("Expected err to be nil")
	}

	err = con.Connect()
	if err != nil {
		t.Error("Received error when connecting.")
	}

	assertEqual(t, con.IsActive(), true)

	// Send payload to echo server and wait for data
	// to be read and processed by the AfterReadHook
	payload := []byte("Testing read/write")
	err = con.Write(&payload)
	if err != nil {
		t.Error(err)
	}

	data := <-con.Read
	expectation := "Testing read/write!"
	if string(*data) != expectation {
		t.Errorf("%s != %s", data, expectation)
	}

	close(done)
}

func BenchmarkThroughput(b *testing.B) {
	done := make(chan bool)
	l, err := testutils.EchoServer(done)
	if err != nil {
		b.Fatal(err)
	}

	conf := Config{Endpoint: l.Addr().String()}
	con, err := NewClient(&conf)
	if err != nil {
		b.Fatal("Expected err to be nil")
	}

	err = con.Connect()
	defer con.Close()
	if err != nil {
		b.Fatal("Received error connecting to endpoint during benchmark.")
	}

	payloadSize := 32 * 1024
	payload := make([]byte, payloadSize) // 32 KB of random bytes; twice the read-buffer size
	rand.Read(payload)
	nextIter := make(chan int)

	for i := 0; i < b.N; i++ {
		go func(conn *Client, nextIter chan int, i int) {
			totalBytes := 0
			for data := range conn.Read {
				totalBytes += len(*data)
				if totalBytes == payloadSize {
					break
				}
			}
			nextIter <- i
		}(con, nextIter, i)
		con.Write(&payload)
		<-nextIter
	}
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
