[![Go Report Card](https://goreportcard.com/badge/github.com/joedursun/EventedConnection)](https://goreportcard.com/report/github.com/joedursun/EventedConnection)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/joedursun/EventedConnection)](https://pkg.go.dev/github.com/joedursun/EventedConnection)

**Disclaimer: this is under active development. The API changes frequently for now and there are few tests until the API is more stable. This is not suitable for production yet!**

## EventedConnection

EventedConnection is a wrapper around `net.Conn` that aims to simplify maintaining a long running connection.
TCP connections can become tedious to handle when being read from or written to in multiple
goroutines, and the reconnect logic can often lead to attempts to write to an old connection.

EventedConnection provides a way to safely read and write to a TCP connection using native Go channels
in a thread-safe way. For example, calling `Close()` on the connection will close the TCP connection
and any subsequent calls will be ignored, so calling `Close()` in any number of goroutines will not
cause a panic.

### Performance

Benchmarks are included as part of the test suite so you can verify if eventedconnection will be suitable for your needs.

When tested on a 3.1 GHz Dual-Core Intel Core i5 2017 Macbook Pro it was able to write and subsequently read 32 KB of data in `~77500ns` (or `0.0000775s`) to localhost. Of course when using this to connect to remote hosts there will be much higher latency and other bandwidth constraints, but this serves as evidence of the low overhead cost of this library.

### Event hooks

EventedConnection provides the event hooks whose signatures can be found in `config.go`:
- `AfterReadHook`
- `AfterConnectHook`
- `BeforeDisconnectHook`
- `OnErrorHook`

Please refer to their docs for more information.

### Basic usage

Here is a simple example of how to open a connection, send the phrase "Hello world!" and close the connection.

```go
package main

import (
  "fmt"
  "github.com/joedursun/EventedConnection"
)

func sendHelloWorld() {
  conf := eventedconnection.Config{ Endpoint: "localhost:5111" }
  con, err := eventedconnection.NewClient(conf)

  if err != nil {
    fmt.Println(err)
    return
  }

  err = con.Connect()
  if err != nil {
    fmt.Println(err)
    return
  }

  msg := []byte("Hello world!")
  con.Write(&msg)
  con.Close()
}

```

For a more advanced example, here we open the connection, overwrite the read data, and close the connection.

```go
import (
  "fmt"
  "github.com/joedursun/EventedConnection"
)

func readFromConnection() {
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

  con, err := eventedconnection.NewClient(conf)
  if err != nil {
    fmt.Println(err)
    return
  }

  err = con.Connect()
  if err != nil {
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
