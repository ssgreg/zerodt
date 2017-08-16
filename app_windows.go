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
