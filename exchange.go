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
	e.mutex.Lock()
	defer e.mutex.Unlock()

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
		if equalTCPAddr(addr, safeAddrToTCPAddr(pr.l.Addr())) {
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
