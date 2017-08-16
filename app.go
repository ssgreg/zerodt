// +build linux darwin

package zerodt

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var (
	// Get original working directory just on start to reduce
	// possibility of calling `os.Chdir` by somebody.
	originalWD, _ = os.Getwd()
)

// App TODO
type App struct {
	served               sync.WaitGroup
	servers              []*http.Server
	WaitForParentTimeout time.Duration
	WaitForChildTimeout  time.Duration
}

type readyMsg struct {
	SendConfirmation bool
}

type shutdownConfirmationMsg struct {
}

// NewApp TODO
func NewApp(servers ...*http.Server) *App {
	a := &App{servers: servers, WaitForParentTimeout: 0, WaitForChildTimeout: time.Second * 60}
	// Need to be sure all servers are serving before calling shutdown.
	a.served.Add(len(a.servers))
	return a
}

// synchronous
func (a *App) shutdown() {
	// Wait for all servers to start serving to avoid race conditions
	// connected with shutdown. 'Shutdown' must be called only if server
	// has already started or it does nothing.
	logger.Printf("ZeroDT: shutdown servers...")
	a.served.Wait()

	var wg sync.WaitGroup
	wg.Add(len(a.servers))

	// Shutdown all servers in parallel
	for _, s := range a.servers {
		go func(s *http.Server) {
			defer wg.Done()
			err := s.Shutdown(context.Background())
			logger.Printf("ZeroDT: server '%v' is shutdown with: '%v'", s.Addr, err)
		}(s)
	}

	wg.Wait()
}

func (a *App) interceptSignals(ctx context.Context, wg *sync.WaitGroup, e *exchange) {
	defer logger.Printf("ZeroDT: stop intercepting signals")
	defer wg.Done()

	signals := make(chan os.Signal, 10)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR2)
	defer signal.Stop(signals)

CatchSignals:
	for {
		select {
		// Exit.
		case <-ctx.Done():
			return
		// OS signal.
		case s := <-signals:
			logger.Printf("ZeroDT: %v signal...", s)
			switch s {
			// Shutdown servers. No exit here.
			case syscall.SIGINT, syscall.SIGTERM:
				a.shutdown()
			// Fork/Exec a child and shutdown.
			case syscall.SIGUSR2:
				_, cp, err := forkExec(e.activeFiles())
				if err != nil {
					logger.Printf("ZeroDT: failed to forkExec: '%v'", err)
					continue CatchSignals
				}
				// Nothing to do with errors.
				protocolActAsParent(cp, a.WaitForChildTimeout, func() {
					a.shutdown()
				})
			}
		}
	}
}

// TODO: think
// - Race condition with sending SIGUSR2 before interceptSignals is starting (need to impelemt sync script for systemd that waits for the new app using http calls or pid)

// Serve TODO
func (a *App) Serve() error {
	inherited, cp, err := inherit()
	if err != nil {
		logger.Printf("ZeroDT: failed to inherit listeners with: '%v'", err)
		return err
	}
	e := newExchange(inherited)
	logger.Printf("ZeroDT: serving with pid='%d', inherited='%s'", os.Getpid(), formatInherited(e))

	// Signals wait group.
	var sigWG sync.WaitGroup
	sigWG.Add(1)
	sigCtx, sigCancelFunc := context.WithCancel(context.Background())
	go a.interceptSignals(sigCtx, &sigWG, e)

	// Servers 'Listen' wait group
	var startWG sync.WaitGroup
	startWG.Add(len(a.servers))
	// Servers 'Serve' wait group.
	var srvWG sync.WaitGroup
	srvWG.Add(len(a.servers))
	// Waiting for parent wait group.
	var parentWG sync.WaitGroup
	parentWG.Add(1)

	for _, s := range a.servers {
		go func(s *http.Server) {
			defer srvWG.Done()
			l, err := e.acquireOrCreateListener("tcp", s.Addr)
			startWG.Done()
			if err != nil {
				// TODO: error channel
				logger.Printf("ZeroDT: failed to listen on '%v' with %v", s.Addr, err)
				return
			}
			parentWG.Wait()

			err = s.Serve(&notifyListener{Listener: tcpKeepAliveListener{l}, wg: &a.served})
			logger.Printf("ZeroDT: server '%v' has finished serving with %v", s.Addr, err)
		}(s)
	}

	// Wait for all listeners to start listening.
	startWG.Wait()

	if cp != nil {
		protocolActAsChild(cp, a.WaitForParentTimeout)
	}

	// Allow serverse's goroutines to start serving
	parentWG.Done()

	// Wait for all server's at first. They may fail or be stopped by
	// calling 'Shutdown'.
	srvWG.Wait()

	// Stop intercepting signals and wait for it's goroutine.
	sigCancelFunc()
	sigWG.Wait()

	return nil
}

// forkExec starts another process of youself and passes active
// listeners to a child to perform socket activation.
func forkExec(files []*os.File) (int, *PipeJSONMessenger, error) {
	// Get the path name for the executable that started the current process.
	path, err := os.Executable()
	if err != nil {
		return -1, nil, err
	}
	// @TODO: remove
	// Fix the path name after the evaluation of any symbolic links.
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return -1, nil, err
	}

	cr, cw, err := os.Pipe()
	if err != nil {
		return -1, nil, err
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return -1, nil, err
	}
	files = append(files, cr, pw)

	// Start the original executable with the original working directory.
	process, err := os.StartProcess(path, os.Args, &os.ProcAttr{
		Dir:   originalWD,
		Env:   prepareEnv(len(files)),
		Files: append([]*os.File{os.Stdin, os.Stdout, os.Stderr}, files...),
	})
	if err != nil {
		return -1, nil, err
	}

	return process.Pid, ListenPipe(pr, cw), nil
}

// formatInherited prints info about inherited listeners to a string.
func formatInherited(e *exchange) string {
	result := "["
	for i, pr := range e.inherited {
		if i != 0 {
			result += ", "
		}
		result += fmt.Sprintf("%v", pr.l.Addr())
	}
	result += "]"
	return result
}

func protocolActAsParent(m *PipeJSONMessenger, timeout time.Duration, shutdownFn func()) error {
	defer m.Close()
	// Set a timeout for the whole dialog.
	m.SetDeadline(time.Now().Add(timeout))
	// Child->Parent, ready message
	logger.Printf("ZeroDT: waiting for child to start (ready signal)...")
	r := readyMsg{}
	err := m.Recv(&r)
	if err != nil {
		logger.Printf("ZeroDT: Parent<=>Child communication failed with: '%v'", err)
		return err
	}
	// Shutdown callback.
	shutdownFn()
	// Parent->Child, confirmation message
	if !r.SendConfirmation {
		return nil
	}
	logger.Printf("ZeroDT: sending confirmation to child...")
	err = m.Send(shutdownConfirmationMsg{})
	if err != nil {
		logger.Printf("ZeroDT: Parent<=>Child communication failed with: '%v'", err)
		return err
	}
	return nil
}

func protocolActAsChild(m *PipeJSONMessenger, timeout time.Duration) {
	defer m.Close()
	// Child->Parent, ready message
	logger.Printf("ZeroDT: sending ready to a parent...")
	m.SetDeadline(time.Now().Add(time.Second * 5))
	err := m.Send(readyMsg{SendConfirmation: timeout != 0})
	if err != nil {
		logger.Printf("ZeroDT: Parent<=>Child communication failed with: '%v'", err)
		return
	}
	if timeout == 0 {
		return
	}
	// Parent->Child, confirmed message
	logger.Printf("ZeroDT: waiting for parent to shutdown...")
	r := shutdownConfirmationMsg{}
	m.SetDeadline(time.Now().Add(timeout))
	err = m.Recv(&r)
	if err != nil {
		logger.Printf("ZeroDT: Parent<=>Child communication failed with: '%v'", err)
		return
	}
}
