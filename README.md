[![Go Report Card](https://goreportcard.com/badge/github.com/joedursun/EventedConnection)](https://goreportcard.com/report/github.com/joedursun/EventedConnection)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/joedursun/EventedConnection)](https://pkg.go.dev/github.com/joedursun/EventedConnection)
[![Build Status](https://travis-ci.org/joedursun/EventedConnection.svg?branch=main)](https://travis-ci.org/joedursun/EventedConnection)

## EventedConnection

eventedconnection is a wrapper around `net.Conn` that aims to simplify maintaining a long running connection.
TCP connections can become tedious to handle when being read from or written to in multiple
goroutines, and the reconnect logic can often lead to attempts to write to an old connection.

eventedconnection provides a way to safely read and write to a TCP connection using native Go channels
in a thread-safe way. For example, calling `Close()` on the connection will close the TCP connection
and any subsequent calls will be ignored, so calling `Close()` in any number of goroutines will not
cause a panic.

### Performance

Benchmarks are included as part of the test suite so you can verify if eventedconnection will be suitable for your needs.

When tested on a 3.1 GHz Dual-Core Intel Core i5 2017 Macbook Pro it was able to write and subsequently read 32 KB of data in `~77500ns` (or `0.0000775s`) to localhost. Of course when using this to connect to remote hosts there will be much higher latency and other bandwidth constraints, but this shows eventedconnection is fast enough for most applications.

### Event hooks

EventedConnection provides the event hooks whose signatures can be found in `config.go`:
- `AfterReadHook`
- `AfterConnectHook`
- `BeforeDisconnectHook`
- `OnErrorHook`

Please refer to their docs for more information.

### Basic usage

Here is a simple example of how to open a connection, send the phrase "Hello world!" and reconnect in the event of a connection error.

```go
package main

import (
	"fmt"

	eventedconnection "github.com/joedursun/EventedConnection"
)

func main() {
	conf := eventedconnection.Config{Endpoint: "localhost:5111"}
	con, _ := eventedconnection.NewClient(&conf)

	if err := con.Connect(); err != nil {
		return
	}

	message := []byte("Hello, world!")
	con.Write(&message)

	reconnectionsLeft := 5
	for reconnectionsLeft > 0 {
		select {
		case <-con.Disconnected:
			reconnectionsLeft--
			if err := con.Reconnect(); err != nil {
				return
			}
		case data := <-con.Read:
			if data != nil {
				fmt.Println(string(*data))
			}
		}
	}
}

```

For a more advanced example, here we open the connection, overwrite the read data, and close the connection.

```go
package main

import (
	"fmt"

	eventedconnection "github.com/joedursun/EventedConnection"
)

func main() {
	// NewConfig initializes a config struct with defaults
	// NewClient will provide defaults if the config
	// doesn't set them, so using NewConfig is optional
	conf := eventedconnection.NewConfig()
	conf.Endpoint = "localhost:5111"
	conf.AfterReadHook = func(data []byte) ([]byte, error) {
		fmt.Println("Data before processing ", string(data))
		processed := []byte("Processed data!")
		return processed, nil
	}

	if con, err := eventedconnection.NewClient(conf); err != nil {
		fmt.Println(err)
		return
	}

	if err = con.Connect(); err != nil {
		fmt.Println(err)
		return
	}

	// the following loop closes the connection after reading one
	// packet but since con.Close() is idempotent we can safely
	// add this deferred Close()
	defer con.Close()

	for {
		select {
		case <-con.Disconnected:
			fmt.Println("Disconnected.")
			return
		case data := <-con.Read:
			if data != nil {
				fmt.Println(string(*data))
			}
			fmt.Println("Closing the connection.")
			con.Close()
		}
	}
}
```

### Testing

In order to test connecting/reading/writing to an endpoint, the tests make use of a simple `net.Listener` which listens on a randomly chosen available port. If you plan to run the tests be sure to allow this behavior or you'll see many spurious failures.

To run the tests: `go test -v`

If you want to run the benchmarks along with the tests: `go test -v -bench=.`

If you only want to run the benchmarks: `go test -v -run=Bench -bench=.`
