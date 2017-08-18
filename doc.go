/*
Package zerodt offers a zero downtime restart and a graceful shutdown for HTTP servers.

The simplest way to use ZeroDT is to pass your http.Server to the NewApp() function and call ListenAndServe() for an object it returns:

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

For more details visit https://github.com/ssgreg/zerodt
*/
package zerodt
