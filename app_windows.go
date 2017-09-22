// Copyright 2017 Grigory Zubankov. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
//

package zerodt

import (
	"context"
	"net/http"
	"time"
)

// TODO: use os.Interrupt

// App specifies functions to control passed HTTP servers.
type App struct {
	PreServeFn         func(inherited bool) error
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

	return err
}

// Shutdown calls Shutdown for all server and returns first error if happens or nil.
func (a *App) Shutdown() error {
	errs := make(chan error)
	for _, server := range a.servers {
		server := server
		go func() {
			errs <- server.Shutdown(context.Background())
		}()
	}

	var err error
	for i := 0; i < len(a.servers); i++ {
		e := <-errs
		if e != nil && err == nil {
			err = e
		}
	}

	return err
}

// SetWaitParentShutdownTimeout does nothing
func (a *App) SetWaitParentShutdownTimeout(d time.Duration) {
}
