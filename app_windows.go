// Copyright 2017 Grigory Zubankov. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
//

package zerodt

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"
)

// App specifies functions to control passed HTTP servers.
type App struct {
	PreServeFn         func(inherited bool) error
	PreShutdownFn      func()
	CompleteShutdownFn func()
	PreParentExitFn    func()
	servers            []*http.Server
}

// NewApp returns a new instance of App.
func NewApp(servers ...*http.Server) *App {
	return &App{nil, nil, nil, servers}
}

// ListenAndServe calls ListenAndServe for all servers and returns first error if happens or nil.
func (a *App) ListenAndServe() error {
	var sigWG sync.WaitGroup
	sigWG.Add(1)
	sigCtx, sigCancelFunc := context.WithCancel(context.Background())
	go a.handleSignals(sigCtx, &sigWG)

	errs := make(chan error)
	for _, server := range a.servers {
		server := server
		go func() {
			errs <- server.ListenAndServe()
		}()
	}

	var err error
	for i := 0; i < len(a.servers); i++ {
		e := <-errs
		if e != nil && e != http.ErrServerClosed && err == nil {
			err = e
		}
	}

	sigCancelFunc()
	sigWG.Wait()

	return err
}

// Shutdown calls Shutdown for all server and returns first error if happens or nil.
func (a *App) Shutdown() {
	errs := make(chan error)
	for _, server := range a.servers {
		server := server
		go func() {
			err := server.Shutdown(context.Background())
			if err != nil {
				logger.Printf("server %s has been shutdown with: %v", server.Addr, err)
			} else {
				logger.Printf("server %s has been shutdown", server.Addr)
			}
			errs <- err
		}()
	}

	for i := 0; i < len(a.servers); i++ {
		<-errs
	}
}

// SetWaitParentShutdownTimeout does nothing.
func (a *App) SetWaitParentShutdownTimeout(d time.Duration) {
}

// SetWaitChildTimeout does nothing.
func (a *App) SetWaitChildTimeout(d time.Duration) {
}

func (a *App) handleSignals(ctx context.Context, wg *sync.WaitGroup) {
	defer logger.Printf("stop handling signals")
	defer wg.Done()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)

	for {
		select {
		// Exit.
		case <-ctx.Done():
			return
		// OS signal.
		case s := <-signals:
			logger.Printf("signal: %v", s)
			switch s {
			// Shutdown servers. No exit here.
			case os.Interrupt:
				a.Shutdown()
			}
		}
	}
}
