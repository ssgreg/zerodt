// Copyright 2017 Grigory Zubankov. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
//
// +build linux darwin

package zerodt

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmptyExchange(t *testing.T) {
	e := newExchange(nil)
	assert.Empty(t, e.inherited)
	assert.Empty(t, e.activeFiles())
	assert.Equal(t, false, e.didInherit())

	l := newTCPListener(t)
	err := e.activateListener(l)
	require.NoError(t, err)
	assert.Empty(t, e.inherited)
	assert.Equal(t, 1, len(e.active))
	assert.NoError(t, l.Close())

	assert.Equal(t, 1, len(e.activeFiles()))
}

func TestExchange(t *testing.T) {
	l := newTCPListener(t)
	f, err := l.File()
	require.NoError(t, err)

	e := newExchange([]*fileListenerPair{{l, f}})
	assert.Equal(t, 1, len(e.inherited))
	assert.Empty(t, e.activeFiles())
	assert.Equal(t, true, e.didInherit())

	l1 := e.acquireListener(l.Addr().(*net.TCPAddr))
	require.NoError(t, err)
	assert.NotNil(t, l1)
	assert.Empty(t, e.inherited[0])
	assert.Equal(t, 1, len(e.active))
	assert.Equal(t, 1, len(e.activeFiles()))
}

func newTCPListener(t *testing.T) *net.TCPListener {
	addr, err := net.ResolveTCPAddr("tcp", ":0")
	require.NoError(t, err)
	l, err := net.ListenTCP("tcp", addr)
	require.NoError(t, err)
	return l
}
