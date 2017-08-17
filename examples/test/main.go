package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/ssgreg/zerodt"
)

func sleep(w http.ResponseWriter, r *http.Request) {
	logrus.Printf("Ready to handle message %s", r.RequestURI)

	duration, err := time.ParseDuration(r.FormValue("duration"))
	if err != nil {
		http.Error(w, err.Error(), 400)
	}

	time.Sleep(duration)
	_, err = fmt.Fprintf(w, "%d", os.Getpid())
	logrus.Printf("Handled message %s, %v", r.RequestURI, err)
}

func main() {
	zerodt.SetLogger(logrus.StandardLogger())

	r := mux.NewRouter()
	r.Path("/sleep").Methods("GET").HandlerFunc(sleep)

	a := zerodt.NewApp(&http.Server{Addr: "127.0.0.1:8081", Handler: r}, &http.Server{Addr: "127.0.0.1:8082", Handler: r})
	a.SetWaitParentShutdownTimeout(time.Second * 120)

	err := a.Serve()
	logrus.Println("Exit serve:", err)
	logrus.Println("That's all Folks!")
}
