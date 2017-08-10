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
		Transport: &http.Transport{IdleConnTimeout: time.Second * 3},
	}
	setEnv("", "")
	return &run{t: t, client: &client, port: strconv.Itoa(port)}
}

func (d *run) start() {
	fmt.Println("===== start")

	d.swg.Add(1)

	// Start an http server.
	cmd := exec.Command(os.Args[0], "-port", d.port)
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
	for i := 0; i < 10; i++ {
		r, err := d.client.Get("http://localhost:" + d.port)
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

func sendMessage(t *testing.T, wg *sync.WaitGroup, client *http.Client, port string, delay, pid int) {
	defer wg.Done()

	r, err := client.Get(fmt.Sprintf("http://localhost:%s/sleep?duration=%ds", port, delay))
	fmt.Println("===== request finished")
	require.NoError(t, err)
	body, err := ioutil.ReadAll(r.Body)
	require.NoError(t, err)
	pidNew, err := strconv.Atoi(string(body))
	require.NoError(t, err)
	assert.Equal(t, pid, pidNew)
}

func (d *run) send() {
	fmt.Println("===== send")

	d.swg.Add(2)

	go sendMessage(d.t, &d.swg, d.client, d.port, 0, d.lastProcess().Pid)
	go sendMessage(d.t, &d.swg, d.client, d.port, 3, d.lastProcess().Pid)
	time.Sleep(time.Millisecond * 1)
}

func (d *run) stop() {
	fmt.Println("===== stop")

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

func TestTrippleRestart(t *testing.T) {
	t.Parallel()
	d := newRun(t, 2606)
	d.start()
	d.send()
	d.restart()
	d.send()
	d.restart()
	d.send()
	d.restart()
	d.send()
	d.stop()
	d.wait()
}

func TestOneMoreTrippleRestart(t *testing.T) {
	t.Parallel()
	d := newRun(t, 2607)
	d.start()
	d.send()
	d.restart()
	d.send()
	d.restart()
	d.send()
	d.restart()
	d.send()
	d.stop()
	d.wait()
}

//
// A server live here.
//

func root(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%d", os.Getpid())
}

func sleep(w http.ResponseWriter, r *http.Request) {
	duration, err := time.ParseDuration(r.FormValue("duration"))
	if err != nil {
		http.Error(w, err.Error(), 400)
	}

	time.Sleep(duration)
	fmt.Fprintf(w, "%d", os.Getpid())
	logger.Printf("Handled message %d:%d", duration, os.Getpid())
}

func startHTTPServer() {
	SetLogger(log.New(os.Stdout, "", log.LstdFlags))

	var port string
	flag.StringVar(&port, "port", "2607", "a port to bind to")
	flag.Parse()

	r := mux.NewRouter()
	r.Path("/").Methods("GET").HandlerFunc(root)
	r.Path("/sleep").Methods("GET").HandlerFunc(sleep)

	// Force timeout. 10 seconds is enough.
	go func() {
		time.Sleep(time.Second * 10)
		p, err := os.FindProcess(os.Getpid())
		logger.Printf("Timeout: kill %v", p.Pid)
		if err == nil {
			p.Kill()
		}
	}()

	a := NewApp(&http.Server{Addr: ":" + port, Handler: r})
	a.Serve()
}
