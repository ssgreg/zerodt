// +build linux darwin

package zerodt

import (
	"bytes"
	"net"
	"os"
	"sync"
	"syscall"
)

// A Lyrical Digression.
//
// During the early alpha version of the package, I discovered an issue
// connected with TCPListener's File() function.
//
// Sometimes my demo http server hang up after successful Shutdown call.
// The only way to exit from server.Serve() was extra http request that
// was always finished with an error to a client.
//
// A long investigation has revealed that the cause of a race condition
// was a code with a strange comment (net/fd_unix.go, dup()):
//
//	// We want blocking mode for the new fd, hence the double negative.
//	// This also puts the old fd into blocking mode, meaning that
//	// I/O will block the thread instead of letting us use the epoll server.
//	// Everything will still work, just with more threads.
//	if err = syscall.SetNonblock(ns, false); err != nil {
//		return nil, os.NewSyscallError("setnonblock", err)
//	}
//
// This code put my listener's fd into blocking mode and force an issue
// to happen. Unfortunately SetNonblock() did nothing while server.Server()
// was calling Accept() function.
//
// To avoid the race condition I decided to duplicate a listener just
// before server.Server() and put it into non-blocking mode by myself.
// This forces me to open and keep additional file descriptor per each
// listener, but it's worth it.

// socketPair describes a pair of TCPListener and File.
type socketPair struct {
	l *net.TCPListener
	f *os.File
}

// exchange - TODO
type exchange struct {
	inherited []*socketPair
	active    []*os.File
	mutex     sync.Mutex
}

func newExchange() (*exchange, error) {
	// Are there some sockets to inherit?
	fds, err := listenFds()
	if err != nil {
		return nil, err
	}
	// Start to listen them.
	pairs := make([]*socketPair, len(fds))
	for i, fd := range fds {
		f, err := newFileOnSocket(fd)
		if err != nil {
			return nil, err
		}
		l, err := newFileTCPListener(f)
		if err != nil {
			return nil, err
		}
		pairs[i] = &socketPair{l, f}
	}
	return &exchange{inherited: pairs}, nil
}

func (e *exchange) didInherit() bool {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	return len(e.inherited) > 0
}

func (e *exchange) activeFiles() []*os.File {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// Make a separate slice of active files (duped listeners)
	active := make([]*os.File, len(e.active))
	copy(active, e.active)
	return active
}

func (e *exchange) acquireListener(addr *net.TCPAddr) *net.TCPListener {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	for i, pr := range e.inherited {
		if pr == nil {
			// This socket pair is already acquired.
			continue
		}
		if equalTCPAddr(addr, safeAddrToTCPAddr(pr.l.Addr())) {
			// Acquire the socket pair: move it to the active array
			e.active = append(e.active, pr.f)
			e.inherited[i] = nil
			return pr.l
		}
	}
	return nil
}

func (e *exchange) addListener(l *net.TCPListener) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// Duplicate a listener. Exchange needed a copy of a listener to be
	// able to pass it to a child.
	f, err := l.File()
	if err != nil {
		return err
	}
	// Note! Read a comment on the top of the file to understand
	// what's going on.
	err = syscall.SetNonblock(int(f.Fd()), true)
	if err != nil {
		return err
	}

	// Add a file to the active array. Only files in active array
	// will be passed to a child.
	e.active = append(e.active, f)
	return nil
}

func equalTCPAddr(l *net.TCPAddr, r *net.TCPAddr) bool {
	return true &&
		// Need to match zones,
		l.Zone == r.Zone &&
		// ports,
		l.Port == r.Port &&
		// and IPs.
		// Note! The only way to compare IPs directly, is to convert
		// them to a 16-byte representation form before.
		bytes.Equal(l.IP.To16(), r.IP.To16())
}

func safeAddrToTCPAddr(a net.Addr) *net.TCPAddr {
	switch la := a.(type) {
	case *net.TCPAddr:
		return la
	default:
		panic("Not a TCPAddr!")
	}
}
