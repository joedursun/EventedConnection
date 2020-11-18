## EventedConnection

EventedConnection is a wrapper around a TCP connection that aims to simplify maintaining a connection.
TCP connections can often become tedious to maintain when being read from or written to in multiple
goroutines, and the reconnect logic can lead to attempts to write to an old connection.

EventedConnection provides a way to safely read and write to a TCP connection using native Go channels
in a thread-safe way. For example, calling `Close()` on the connection will close the TCP connection
and any subsequent calls will be ignored, so calling `Close()` in any number of goroutines will not
cause a panic.

### Lifecycle hooks

EventedConnection provides the lifecycle hooks whose signatures can be found in `connection/config.go`:
- `AfterReadHook`: called after reading data from the connection. Can be used to modify or filter received data or to throw an error if receiving something unexpected.

### Usage

```go
package main

import (
  "fmt"
  "github.com/joedursun/EventedConnection/connection"
)

func main() {
  conf := connection.Config{
    Endpoint: "localhost:5111",
    ConnectionTimeout: 3,
    AfterReadHook: func(data []byte) ([]byte, error) {
      fmt.Println("Data before processing ", string(data))
      data = []byte("Processed data!")
      return data, nil
    },
  }

  con, err := connection.NewEventedConnection(&conf)
  if err != nil {
    fmt.Println(err)
  }

  con.Connect()
  for {
    select {
    case data := <-con.Read:
      fmt.Println(string(*data))
      fmt.Println("Closing the connection.")
      con.Close()
    case <-con.Disconnected:
      fmt.Println("Disconnected.")
      return
    case <-con.Canceled:
      fmt.Println("Canceled.")
      return
    }
  }
}
```
