// Copyright 2017 Grigory Zubankov. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
//

package zerodt

import (
	"net/http"
)

// TODO: use os.Interrupt

// App TODO
type App struct {
	servers []*http.Server
}

// NewApp TODO
func NewApp(servers ...*http.Server) *App {
	return &App{servers}
}

// Serve TODO
func (a *App) Serve() error {
	panic("Implement")
}
