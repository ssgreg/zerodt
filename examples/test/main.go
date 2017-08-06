package main

import (
	"fmt"
	"net/http"
	"os"
	"ssgreg/zerodt"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%v", os.Getpid())
	//	time.Sleep(time.Second * 20)
	fmt.Println("ready!")
}

func main() {
	zerodt.SetLogger(logrus.StandardLogger())

	r := mux.NewRouter()
	r.Methods("GET").HandlerFunc(handler)

	a := zerodt.NewApp(&http.Server{Addr: "127.0.0.1:8081", Handler: r})
	a.Serve()

	zerodt.Greg()
	fmt.Printf("That's all Folks!")
}
