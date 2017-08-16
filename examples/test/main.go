package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/ssgreg/zerodt"
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
	zerodt.SetLogger(logrus.StandardLogger())

	// r := mux.NewRouter()
	// r.Path("/sleep").Methods("GET").HandlerFunc(sleep)

	// a := zerodt.NewApp(&http.Server{Addr: "127.0.0.1:8081", Handler: r}, &http.Server{Addr: "127.0.0.1:8082", Handler: r})
	// a.Serve()

	// srv := http.Server{Addr: "127.0.0.1:8081", Handler: r}

	// addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8081")
	// l, _ := net.ListenTCP("tcp", addr)

	// err := srv.Shutdown(context.Background())
	// fmt.Println(err)
	// srv.Serve(l)

	// var wg sync.WaitGroup
	// wg.Add(2)

	// fmt.Println("Started!")

	// ch := make(chan struct{})

	// go func(ch chan struct{}) {
	// 	defer wg.Done()
	// 	fmt.Println("r")
	// 	<-ch
	// 	fmt.Println("1")
	// }(ch)

	// go func(ch chan struct{}) {
	// 	defer wg.Done()
	// 	fmt.Println("r")
	// 	<-ch
	// 	fmt.Println("2")
	// }(ch)

	// time.Sleep(time.Second * 1)
	// close(ch)

	// <-ch
	// <-ch
	// <-ch
	// <-ch

	// wg.Wait()

	fmt.Printf("That's all Folks!")
}
