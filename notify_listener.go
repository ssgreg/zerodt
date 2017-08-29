// Copyright 2017 Grigory Zubankov. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
//
// +build linux darwin

package zerodt

import (
	"net"
	"sync"
)

// There is a bug in go that described in details here:
// https://github.com/golang/go/issues/20239
//
// In a nutshell, if the shutdown is happening immediately after the Serve()
// is started there's a race and Shutdown() may be called on a server which
// has not started. It means a server will not be shutdown.
//
// A workaround is to wait just before Shutdown() call for the first Accept()
// call made by a Serve() on a passed listener.
//
//   var wg sync.WaitGroup
//   l, _ := net.Listen("tcp", ":8080")
//   once := &doneOnce(wg: &wg)
//
//   go s.Serve(&notifyListener{Listener: l, doneOnce: once})
//
//   wg.Wait()
//   // It's safe to call shutdown here
//   s.Shutdown(context.Background())
//

type doneOnce struct {
	wg   *sync.WaitGroup
	once sync.Once
}

func (n *doneOnce) Done() {
	n.once.Do(func() {
		n.wg.Done()
	})
}

type notifyListener struct {
	net.Listener
	doneOnce *doneOnce
}

func (l *notifyListener) Accept() (net.Conn, error) {
	l.doneOnce.Done()
	return l.Listener.Accept()
}
