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
)

var (
	// Get original working directory just on start to reduce
	// possibility of calling `os.Chdir` by somebody.
	originalWD, _ = os.Getwd()
)

// App TODO
type App struct {
	servers  []*http.Server
	exchange *exchange
}

// NewApp TODO
func NewApp(servers ...*http.Server) *App {
	// @todo: move to Serve
	inherited, err := inheritedFileListenerPairs()
	if err != nil {
		logger.Printf("ZeroDT: failed to inherit listeners with: '%v'", err)
		panic(err)
	}
	e := newExchange(inherited)
	logger.Printf("ZeroDT: started for pid=%d with inherited=%s", os.Getpid(), formatInherited(e))
	return &App{servers, e}
}

// synchronous
func (a *App) shutdown() {
	var wg sync.WaitGroup
	wg.Add(len(a.servers))

	// Shutdown all servers in parallel
	for _, s := range a.servers {
		go func(s *http.Server) {
			defer wg.Done()
			err := s.Shutdown(context.Background())
			logger.Printf("ZeroDT: server '%v' is shutdown with %v", s.Addr, err)
		}(s)
	}

	wg.Wait()
}

func (a *App) interceptSignals(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR2)
	defer signal.Stop(signals)

	for {
		select {
		// OS signal.
		case s := <-signals:
			switch s {
			case syscall.SIGINT, syscall.SIGTERM:
				logger.Printf("ZeroDT: termination signal, shutdown servers...")
				a.shutdown()

			case syscall.SIGUSR2:
				// @TODO: make synchronous
				logger.Printf("ZeroDT: activation signal, starting another process...")
				pid, err := a.startAnotherProcess()
				if err != nil {
					// TODO: send to error channel
				}
				logger.Printf("ZeroDT: child '%d' successfully started", pid)
			}
		// Cancel, no need to shutdown servers.
		case <-ctx.Done():
			return
		}
	}
}

func (a *App) killParent() {
	if !a.exchange.didInherit() {
		return
	}
	// If it's systemd - keep it alive. This is possible when
	// 'socket activation' take place.
	if os.Getppid() == 1 {
		return
	}

	logger.Printf("ZeroDT: send termination signal to the parent with pid=%d", os.Getppid())
	// @TODO: to panic or not
	err := syscall.Kill(os.Getppid(), syscall.SIGTERM)
	if err != nil {
		// It does not allowed running both binaries.
		panic(err)
	}
}

// Serve TODO
func (a *App) Serve() error {
	var srvWG sync.WaitGroup
	srvWG.Add(len(a.servers))

	var sigWG sync.WaitGroup
	sigWG.Add(1)

	var startWG sync.WaitGroup
	startWG.Add(len(a.servers))

	sigCtx, cancelFunc := context.WithCancel(context.Background())
	go a.interceptSignals(sigCtx, &sigWG)

	for _, s := range a.servers {
		go func(s *http.Server) {
			defer srvWG.Done()

			l, err := a.exchange.acquireOrCreateListener("tcp", s.Addr)
			startWG.Done()
			if err != nil {
				// TODO: error channel
				logger.Printf("ZeroDT: failed to listen on '%v' with %v", s.Addr, err)
				return
			}

			err = s.Serve(tcpKeepAliveListener{l})
			// Serve always returns a non-nil error.
			logger.Printf("ZeroDT: server '%v' has finished serving with %v", s.Addr, err)
		}(s)
	}

	startWG.Wait()
	// Kill a parent in case the process was started with inherited sockets.
	a.killParent()

	// Wait for all server's at first. They may fail or be stopped by
	// calling 'Shutdown'.
	srvWG.Wait()
	// Stop intercepting signals. No need to shutdown servers in this case.
	cancelFunc()
	// Wait for the last goroutine.
	sigWG.Wait()

	return nil
}

// startAnotherProcess starts another process of youself and passes active
// listeners to a child to perform socket activation.
func (a *App) startAnotherProcess() (int, error) {
	// Get the path name for the executable that started the current process.
	path, err := os.Executable()
	if err != nil {
		return -1, err
	}
	// @TODO: remove
	// Fix the path name after the evaluation of any symbolic links.
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return -1, err
	}

	files := a.exchange.activeFiles()

	// Start the original executable with the original working directory.
	process, err := os.StartProcess(path, os.Args, &os.ProcAttr{
		Dir:   originalWD,
		Env:   prepareEnv(len(files)),
		Files: append([]*os.File{os.Stdin, os.Stdout, os.Stderr}, files...),
	})
	if err != nil {
		return -1, err
	}

	return process.Pid, nil
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
