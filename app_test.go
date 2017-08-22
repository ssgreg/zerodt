// Copyright 2017 Grigory Zubankov. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
//
// +build linux darwin

package zerodt

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	const (
		envTestKey   = "ZERODT_START_HTTP_SERVER"
		envTestValue = "1"
	)
	if os.Getenv(envTestKey) == envTestValue {
		startHTTPServer()
		return
	}
	err := os.Setenv(envTestKey, envTestValue)
	if err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

type run struct {
	t         *testing.T
	processes []*os.Process
	swg       sync.WaitGroup
	client    *http.Client
	port      string
}

func newRun(t *testing.T, port int) *run {
	client := http.Client{
		Transport: &http.Transport{IdleConnTimeout: time.Second * 5, MaxIdleConnsPerHost: 70},
	}
	setEnv("", "")
	return &run{t: t, client: &client, port: strconv.Itoa(port)}
}

func (d *run) start(waitForParent bool) {
	fmt.Println("===== start")

	d.swg.Add(1)

	args := make([]string, 0)
	args = append(args, "-port", d.port)
	if waitForParent {
		args = append(args, "-waitForParent")
	}

	// Start an http server.
	cmd := exec.Command(os.Args[0], args...)
	stdout, err := cmd.StdoutPipe()
	require.NoError(d.t, err)
	err = cmd.Start()
	require.NoError(d.t, err)

	// Copy process's stdout to our stdout.
	go func(t *testing.T, stdout io.ReadCloser) {
		defer d.swg.Done()
		_, err = io.Copy(os.Stdout, stdout)
		fmt.Println("===== stout finished")
		require.NoError(d.t, err)
	}(d.t, stdout)

	// Get new process pid.
	d.waitForProcess(true)
}

func (d *run) waitForProcess(retryErrors bool) {
	for i := 0; i < 1000; i++ {
		fmt.Println("===== ping")
		r, err := d.client.Get("http://localhost:" + d.port)
		fmt.Println("===== pong")
		if err != nil {
			if retryErrors {
				time.Sleep(time.Millisecond * 100)
				continue
			}
			require.NoError(d.t, err)
		}
		body, err := ioutil.ReadAll(r.Body)
		require.NoError(d.t, err)
		pid, err := strconv.Atoi(string(body))
		require.NoError(d.t, err)
		process, err := os.FindProcess(pid)
		require.NoError(d.t, err)
		if len(d.processes) == 0 || d.lastProcess().Pid != process.Pid {
			d.processes = append(d.processes, process)
			return
		}
		time.Sleep(time.Millisecond * 100)
	}
	d.t.Fatal("Process is not started!")
}

func sendMessage(t *testing.T, wg *sync.WaitGroup, client *http.Client, port string, delay, id, pid int) {
	defer wg.Done()

	r, err := client.Get(fmt.Sprintf("http://localhost:%s/sleep?duration=%dms&pid=%d&id=%d", port, delay, pid, id))
	fmt.Println("===== request finished")
	require.NoError(t, err)
	body, err := ioutil.ReadAll(r.Body)
	require.NoError(t, err)
	pidNew, err := strconv.Atoi(string(body))
	require.NoError(t, err)
	assert.Equal(t, pid, pidNew)
}

func sendMessage2(t *testing.T, wg *sync.WaitGroup, client *http.Client, port string, ch chan int, delay, id, pid int) {
	defer wg.Done()

	r, err := client.Get(fmt.Sprintf("http://localhost:%s/sleep?duration=%dms&pid=%d&id=%d", port, delay, pid, id))
	fmt.Println("===== request finished")
	require.NoError(t, err)
	body, err := ioutil.ReadAll(r.Body)
	require.NoError(t, err)
	pidNew, err := strconv.Atoi(string(body))
	require.NoError(t, err)
	assert.Equal(t, pid, pidNew)
	ch <- id
}

var counter int32

func sendMessage1(t *testing.T, client *http.Client, port string, delay, pid int) {
	myCounter := atomic.AddInt32(&counter, 1)

	uri := fmt.Sprintf("http://localhost:%s/sleep?duration=%dms&counter=%d", port, delay, myCounter)

	fmt.Println("========== started", uri)
	r, err := client.Get(uri)
	fmt.Println("========== finished", uri)
	require.NoError(t, err)
	_, err = ioutil.ReadAll(r.Body)
	require.NoError(t, err)
}

func (d *run) send() {
	fmt.Println("===== send")

	d.swg.Add(2)

	go sendMessage(d.t, &d.swg, d.client, d.port, 0, 0, d.lastProcess().Pid)
	go sendMessage(d.t, &d.swg, d.client, d.port, 300, 0, d.lastProcess().Pid)
	time.Sleep(time.Millisecond * 1)
}

func (d *run) sendWithID(ch chan int, short, long, longTimeout int) {
	fmt.Println("===== send")

	d.swg.Add(2)

	go sendMessage2(d.t, &d.swg, d.client, d.port, ch, 0, short, d.lastProcess().Pid)
	go sendMessage2(d.t, &d.swg, d.client, d.port, ch, longTimeout, long, d.lastProcess().Pid)
	time.Sleep(time.Millisecond * 1)
}

func (d *run) stop() {
	fmt.Println("===== stop")

	time.Sleep(time.Second)
	require.NoError(d.t, d.lastProcess().Signal(syscall.SIGTERM))
}

func (d *run) wait() {
	fmt.Println("===== wait")

	d.swg.Wait()
}

func (d *run) restart() {
	fmt.Println("===== restart")

	require.NoError(d.t, d.lastProcess().Signal(syscall.SIGUSR2))
	d.waitForProcess(false)
}

func (d *run) lastProcess() *os.Process {
	l := len(d.processes)
	require.NotEmpty(d.t, l)
	return d.processes[l-1]
}

func TestTrippleRestartStatefull(t *testing.T) {
	d := newRun(t, 2606)

	ch := make(chan int, 100)

	d.start(true)
	d.sendWithID(ch, 0, 1, 2000)
	d.restart()
	d.sendWithID(ch, 0, 1, 2000)
	d.restart()
	d.sendWithID(ch, 0, 1, 2000)
	d.restart()
	d.sendWithID(ch, 0, 1, 2000)
	d.stop()
	d.wait()

	require.Equal(t, 0, <-ch)
	require.Equal(t, 1, <-ch)
	require.Equal(t, 0, <-ch)
	require.Equal(t, 1, <-ch)
	require.Equal(t, 0, <-ch)
	require.Equal(t, 1, <-ch)
	require.Equal(t, 0, <-ch)
	require.Equal(t, 1, <-ch)
}

// func TestHighLoad(t *testing.T) {
// 	t.Parallel()
// 	d := newRun(t, 2606)

// 	d.start(false)

// 	var wg sync.WaitGroup

// 	for i := 0; i < 100; i++ {
// 		wg.Add(1)

// 		go func() {
// 			defer wg.Done()

// 			deadline := time.Now().Add(time.Second * 9)
// 			for time.Now().Sub(deadline) < 0 {
// 				sendMessage1(d.t, d.client, d.port, 0, d.lastProcess().Pid)
// 			}
// 		}()
// 	}

// 	for i := 0; i < 40; i++ {
// 		time.Sleep(time.Millisecond * 100)
// 		d.restart()
// 		d.send()
// 	}
// 	// time.Sleep(time.Second * 15)

// 	wg.Wait()

// 	time.Sleep(time.Second * 1)
// 	d.stop()
// 	d.wait()
// }

func TestTrippleRestartStateless(t *testing.T) {
	d := newRun(t, 2607)

	ch := make(chan int, 100)

	d.start(false)
	d.sendWithID(ch, 0, 1, 5000)
	d.restart()
	d.sendWithID(ch, 0, 1, 5000)
	d.restart()
	d.sendWithID(ch, 0, 1, 5000)
	d.restart()
	d.sendWithID(ch, 0, 1, 5000)
	d.stop()
	d.wait()

	require.Equal(t, 0, <-ch)
	require.Equal(t, 0, <-ch)
	require.Equal(t, 0, <-ch)
	require.Equal(t, 0, <-ch)
	require.Equal(t, 1, <-ch)
	require.Equal(t, 1, <-ch)
	require.Equal(t, 1, <-ch)
	require.Equal(t, 1, <-ch)
}

func TestKillParent(t *testing.T) {
	d := newRun(t, 2608)
	d.start(true)

	_, err := killProcess(d.lastProcess().Pid)
	require.NoError(t, err)
	st, err := d.lastProcess().Wait()
	assert.Nil(t, st)
	assert.EqualValues(t, syscall.ECHILD, underlyingError(err).(syscall.Errno))
}

func underlyingError(err error) error {
	switch err := err.(type) {
	case *os.PathError:
		return err.Err
	case *os.LinkError:
		return err.Err
	case *os.SyscallError:
		return err.Err
	}
	return err
}

//
// A server live here.
//

func root(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%d", os.Getpid())
	logger.Printf("Handled root %d", os.Getpid())
}

func sleep(w http.ResponseWriter, r *http.Request) {
	logger.Printf("Ready to handle message %s", r.RequestURI)

	duration, err := time.ParseDuration(r.FormValue("duration"))
	if err != nil {
		http.Error(w, err.Error(), 400)
	}

	time.Sleep(duration)
	_, err = fmt.Fprintf(w, "%d", os.Getpid())
	logger.Printf("Handled message %s, %v", r.RequestURI, err)
}

func startHTTPServer() {
	SetLogger(log.New(os.Stdout, fmt.Sprintf("[%d] ", os.Getpid()), log.LstdFlags))

	var port string
	var waitForParent bool
	flag.StringVar(&port, "port", "2607", "a port to bind to")
	flag.BoolVar(&waitForParent, "waitForParent", false, "wait for parent before start serving (statefull)")
	flag.Parse()

	logger.Printf("Server started on port=%s with waitForParent=%v\n", port, waitForParent)

	r := mux.NewRouter()
	r.Path("/").Methods("GET").HandlerFunc(root)
	r.Path("/sleep").Methods("GET").HandlerFunc(sleep)

	// Force timeout. 10 seconds is enough.
	go func() {
		time.Sleep(time.Second * 300)
		p, err := os.FindProcess(os.Getpid())
		logger.Printf("Timeout: kill %v\n", p.Pid)
		if err == nil {
			p.Kill()
		}
	}()

	a := NewApp(&http.Server{Addr: ":" + port, Handler: r})
	if waitForParent {
		a.SetWaitParentShutdownTimeout(time.Second * 360)
	}
	a.ListenAndServe()

	logger.Printf("Server finished")
}
