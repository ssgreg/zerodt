# ZeroDT [![Build Status](https://travis-ci.org/ssgreg/zerodt.svg?branch=master)](https://travis-ci.org/ssgreg/zerodt)

Package `ZeroDT` offers a zero downtime restart and a graceful shutdown for HTTP servers.

## Example

The simplest way to use `ZeroDT` is to pass your `http.Server` to the `zerodt.NewApp` function and call `zerodt.ListenAndServe` for a object it returns:

```go
package main

import (
    "io"
    "net/http"
    "time"

    "github.com/ssgreg/zerodt"
)

func hello(w http.ResponseWriter, r *http.Request) {
    io.WriteString(w, "Hello world!")
}

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", hello)

    a := zerodt.NewApp(&http.Server{Addr: "127.0.0.1:8081", Handler: mux})
    a.ListenAndServe()
}
```