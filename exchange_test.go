// +build linux darwin

package zerodt

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmptyExchange(t *testing.T) {
	e := newExchange(nil)
	assert.Empty(t, e.inherited)
	assert.Empty(t, e.activeFiles())
	assert.Equal(t, false, e.didInherit())

	l := newTCPListener(t)
	err := e.activateListener(l)
	assert.NoError(t, err)
	assert.Empty(t, e.inherited)
	assert.Equal(t, 1, len(e.active))
	assert.NoError(t, l.Close())

	assert.Equal(t, 1, len(e.activeFiles()))
}

func TestExchange(t *testing.T) {
	l := newTCPListener(t)
	f, err := l.File()
	assert.NoError(t, err)

	e := newExchange([]*fileListenerPair{{l, f}})
	assert.Equal(t, 1, len(e.inherited))
	assert.Empty(t, e.activeFiles())
	assert.Equal(t, true, e.didInherit())

	l1 := e.acquireListener(l.Addr().(*net.TCPAddr))
	assert.NoError(t, err)
	assert.NotNil(t, l1)
	assert.Empty(t, e.inherited[0])
	assert.Equal(t, 1, len(e.active))

	assert.Equal(t, 1, len(e.activeFiles()))
}

func newTCPListener(t *testing.T) *net.TCPListener {
	addr, err := net.ResolveTCPAddr("tcp", ":0")
	assert.NoError(t, err)
	l, err := net.ListenTCP("tcp", addr)
	assert.NoError(t, err)
	return l
}
