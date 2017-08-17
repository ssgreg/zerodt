// Copyright 2017 Grigory Zubankov. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
//
// +build linux darwin

package zerodt

import (
	"io/ioutil"
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInheritedFileListenerPairs(t *testing.T) {
	setEnv("", "")
	pairs, _, err := inherit()
	require.NoError(t, err)
	assert.Empty(t, pairs)
}

func TestCreateFileListenerPairs(t *testing.T) {
	fd := newSocketTCP(t)
	defer closeFD(t, fd)

	pairs, _, err := inheritWithFDS([]int{fd})
	require.NoError(t, err)
	assert.Equal(t, 1, len(pairs))
	assert.NotNil(t, pairs[0].f)
}

func TestSetDefaultGoSocketOptions(t *testing.T) {
	fd := newSocketTCP(t)
	defer closeFD(t, fd)
	require.NoError(t, syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_BROADCAST, 0))
	require.NoError(t, syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 0))
	require.NoError(t, syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 0))

	require.NoError(t, setDefaultGoSocketOptions(fd))

	v, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_BROADCAST)
	require.NoError(t, err)
	assert.NotEmpty(t, v)
	v, err = syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR)
	require.NoError(t, err)
	assert.NotEmpty(t, v)
	v, err = syscall.GetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY)
	require.NoError(t, err)
	assert.NotEmpty(t, v)
}

func TestIsSocketTCP(t *testing.T) {
	fd := newSocketTCP(t)
	defer closeFD(t, fd)

	// All ok
	res, err := isSocketTCP(fd)
	require.NoError(t, err)
	assert.NotEmpty(t, res)

	// UDP socket fail
	ufd := newSocketUDP(t)
	defer closeFD(t, ufd)
	res, err = isSocketTCP(ufd)
	require.NoError(t, err)
	assert.Empty(t, res)

	// File fd fail
	ffd := newFile(t)
	defer closeFD(t, ffd)
	res, err = isSocketTCP(ffd)
	require.NoError(t, err)
	assert.Empty(t, res)
}

func TestMakeSocketFilename_TCP4(t *testing.T) {
	fd := newSocketTCPn(t, "tcp4")
	defer closeFD(t, fd)

	n, err := makeSocketFilename(fd)
	require.NoError(t, err)
	assert.NotEmpty(t, n)
}

func TestMakeSocketFilename_TCP6(t *testing.T) {
	fd := newSocketTCPn(t, "tcp6")
	defer closeFD(t, fd)

	n, err := makeSocketFilename(fd)
	require.NoError(t, err)
	assert.NotEmpty(t, n)
}

func TestMakeSocketFilename_File(t *testing.T) {
	fd := newFile(t)
	defer closeFD(t, fd)

	_, err := makeSocketFilename(fd)
	assert.Error(t, err)
}

func TestNewFileTCPListener(t *testing.T) {
	fd := newSocketTCP(t)
	defer closeFD(t, fd)

	f, err := newFileOnSocket(fd)
	require.NoError(t, err)

	l, err := newFileTCPListener(f)
	require.NoError(t, err)
	require.NoError(t, l.Close())
}

func newSocketTCP(t *testing.T) int {
	return newSocketTCPn(t, "tcp")
}

func newSocketTCPn(t *testing.T, n string) int {
	addr, err := net.ResolveTCPAddr(n, ":0")
	require.NoError(t, err)
	l, err := net.ListenTCP(n, addr)
	require.NoError(t, err)
	f, err := l.File()
	require.NoError(t, err)
	require.NoError(t, l.Close())
	fd, err := syscall.Dup(int(f.Fd()))
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return fd
}

func newSocketUDP(t *testing.T) int {
	addr, err := net.ResolveUDPAddr("udp", ":0")
	require.NoError(t, err)
	l, err := net.ListenUDP("udp", addr)
	require.NoError(t, err)
	f, err := l.File()
	require.NoError(t, err)
	require.NoError(t, l.Close())
	fd, err := syscall.Dup(int(f.Fd()))
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return fd
}

func newFile(t *testing.T) int {
	f, err := ioutil.TempFile("", "newFile-")
	require.NoError(t, err)
	fd, err := syscall.Dup(int(f.Fd()))
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return fd
}

func closeFD(t *testing.T, fd int) {
	f := os.NewFile(uintptr(fd), "")
	require.NoError(t, f.Close())
}
