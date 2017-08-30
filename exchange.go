// Copyright 2017 Grigory Zubankov. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
//
// +build linux darwin

package zerodt

import (
	"bytes"
	"net"
	"os"
	"sync"
	"syscall"
)

// exchange - TODO
type exchange struct {
	inherited []*fileListenerPair
	active    []*os.File
	mutex     sync.Mutex
}

func newExchange(pairs []*fileListenerPair) *exchange {
	return &exchange{inherited: pairs}
}

// didInherit checks whether exchange contains inherited listeners.
func (e *exchange) didInherit() bool {
	return len(e.inherited) > 0
}

// activeFiles returns an array of files, corresponded to active listeners.
func (e *exchange) activeFiles() []*os.File {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// Make a separate slice of active files (duped listeners)
	active := make([]*os.File, len(e.active))
	copy(active, e.active)
	return active
}

// acquireListener allows to get one of the inherited listeners.
func (e *exchange) acquireListener(addr *net.TCPAddr) *net.TCPListener {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	for i, pr := range e.inherited {
		if pr == nil {
			// This socket pair is already acquired.
			continue
		}
		if equalTCPAddr(addr, pr.l.Addr().(*net.TCPAddr)) {
			// Acquire the socket pair: move it to the active array
			e.active = append(e.active, pr.f)
			e.inherited[i] = nil
			return pr.l
		}
	}
	return nil
}

// activateListener duplicates a listener and keeps duplicate.
// This listener now can be inherited by a child process.
func (e *exchange) activateListener(l *net.TCPListener) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	// Duplicate a listener. Exchange needed a copy of a listener to be
	// able to pass it to a child.
	f, err := l.File()
	if err != nil {
		return err
	}
	// Read 'A Lyrical Digression' in file_listener.go to understand
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

// acquireOrCreateListener is a helper function that acquires an inherited
// listener or creates a new one and adds to an exchange
func (e *exchange) acquireOrCreateListener(netStr, addrStr string) (*net.TCPListener, error) {
	addr, err := net.ResolveTCPAddr(netStr, addrStr)
	if err != nil {
		return nil, err
	}

	// Try to acquire one of inherited listeners.
	l := e.acquireListener(addr)
	if l != nil {
		logger.Printf("listener %v acquired", addr)
		return l, nil
	}

	// Create a new TCP listener and add it to an exchange.
	l, err = net.ListenTCP(netStr, addr)
	if err != nil {
		return nil, err
	}
	err = e.activateListener(l)
	if err != nil {
		l.Close()
		return nil, err
	}
	logger.Printf("listener %v created", addr)

	return l, nil
}

func equalTCPAddr(l *net.TCPAddr, r *net.TCPAddr) bool {
	return true &&
		// Need to match zones,
		l.Zone == r.Zone &&
		// ports,
		l.Port == r.Port &&
		// and IPs.
		bytes.Equal(normalizeIP(l.IP), normalizeIP(r.IP))
}

func normalizeIP(ip net.IP) net.IP {
	// net.IP can be nil after ResolveTCPAddr. The same address
	if ip == nil {
		return net.IPv6zero
	}
	// Note! The only way to compare IPs directly, is to convert
	// them to a 16-byte representation form before.
	return ip.To16()
}
