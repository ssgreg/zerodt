package main

import (
	"fmt"
	"net/http"
	"os"
	"ssgreg/zerodt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

func sleep(w http.ResponseWriter, r *http.Request) {
	duration, err := time.ParseDuration(r.FormValue("duration"))
	if err != nil {
		http.Error(w, err.Error(), 400)
	}

	time.Sleep(duration)
	fmt.Fprintf(w, "%d", os.Getpid())

	logrus.Printf("Handled message %d!", os.Getpid())
}

func main() {
	// @todo move logger to app
	zerodt.SetLogger(logrus.StandardLogger())

	r := mux.NewRouter()
	r.Path("/sleep").Methods("GET").HandlerFunc(sleep)

	a := zerodt.NewApp(&http.Server{Addr: "127.0.0.1:8081", Handler: r}, &http.Server{Addr: "127.0.0.1:8082", Handler: r})
	a.Serve()

	fmt.Printf("That's all Folks!")
}
