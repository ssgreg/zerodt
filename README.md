# ZeroDT

[![GoDoc](https://godoc.org/github.com/ssgreg/zerodt?status.svg)](https://godoc.org/github.com/ssgreg/zerodt)
[![Build Status](https://travis-ci.org/ssgreg/zerodt.svg?branch=master)](https://travis-ci.org/ssgreg/zerodt)

Package `ZeroDT` offers a zero downtime restart and a graceful shutdown for HTTP servers. Key features:

* supported both stateless and stateful servers
* compatible with `systemd's` socket activation
* based on out-of-the-box `http.Server`
* work with any number of servers
* not a framework

## Example

The simplest way to use `ZeroDT` is to pass your `http.Server` to the `zerodt.NewApp` function and call `zerodt.ListenAndServe` for an object it returns:

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

    a := zerodt.NewApp(&http.Server{Addr: ":8081", Handler: mux})
    a.ListenAndServe()
}
```