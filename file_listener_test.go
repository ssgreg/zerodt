// +build linux darwin

package zerodt

import (
	"io/ioutil"
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInheritedFileListenerPairs(t *testing.T) {
	setEnv("", "")
	pairs, err := inheritedFileListenerPairs()
	assert.NoError(t, err)
	assert.Empty(t, pairs)
}

func TestCreateFileListenerPairs(t *testing.T) {
	fd := newSocketTCP(t)
	defer CloseFd(t, fd)

	pairs, err := createFileListenerPairs([]int{fd})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(pairs))
}

func TestSetDefaultGoSocketOptions(t *testing.T) {
	fd := newSocketTCP(t)
	defer CloseFd(t, fd)
	assert.NoError(t, syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_BROADCAST, 0))
	assert.NoError(t, syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 0))
	assert.NoError(t, syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 0))

	assert.NoError(t, setDefaultGoSocketOptions(fd))

	v, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_BROADCAST)
	assert.NoError(t, err)
	assert.NotEmpty(t, v)
	v, err = syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR)
	assert.NoError(t, err)
	assert.NotEmpty(t, v)
	v, err = syscall.GetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY)
	assert.NoError(t, err)
	assert.NotEmpty(t, v)
}

func TestIsSocketTCP(t *testing.T) {
	fd := newSocketTCP(t)
	defer CloseFd(t, fd)

	// All ok
	res, err := isSocketTCP(fd)
	assert.NoError(t, err)
	assert.NotEmpty(t, res)

	// UDP socket fail
	ufd := newSocketUDP(t)
	defer CloseFd(t, ufd)
	res, err = isSocketTCP(ufd)
	assert.NoError(t, err)
	assert.Empty(t, res)

	// File fd fail
	ffd := newFile(t)
	defer CloseFd(t, ffd)
	res, err = isSocketTCP(ffd)
	assert.NoError(t, err)
	assert.Empty(t, res)
}

func TestMakeSocketFilename_Tcp4(t *testing.T) {
	fd := newSocketTCPn(t, "tcp4")
	defer CloseFd(t, fd)

	n, err := makeSocketFilename(fd)
	assert.NoError(t, err)
	assert.NotEmpty(t, n)
}

func TestMakeSocketFilename_Tcp6(t *testing.T) {
	fd := newSocketTCPn(t, "tcp6")
	defer CloseFd(t, fd)

	n, err := makeSocketFilename(fd)
	assert.NoError(t, err)
	assert.NotEmpty(t, n)
}

func TestMakeSocketFilename_File(t *testing.T) {
	fd := newFile(t)
	defer CloseFd(t, fd)

	_, err := makeSocketFilename(fd)
	assert.Error(t, err)
}

func TestNewFileTCPListener(t *testing.T) {
	fd := newSocketTCP(t)
	defer CloseFd(t, fd)

	f, err := newFileOnSocket(fd)
	assert.NoError(t, err)

	l, err := newFileTCPListener(f)
	assert.NoError(t, err)
	assert.NoError(t, l.Close())
}

func newSocketTCP(t *testing.T) int {
	return newSocketTCPn(t, "tcp")
}

func newSocketTCPn(t *testing.T, n string) int {
	addr, err := net.ResolveTCPAddr(n, ":0")
	assert.NoError(t, err)
	l, err := net.ListenTCP(n, addr)
	assert.NoError(t, err)
	f, err := l.File()
	assert.NoError(t, err)
	assert.NoError(t, l.Close())
	fd, err := syscall.Dup(int(f.Fd()))
	assert.NoError(t, err)
	assert.NoError(t, f.Close())
	return fd
}

func newSocketUDP(t *testing.T) int {
	addr, err := net.ResolveUDPAddr("udp", ":0")
	assert.NoError(t, err)
	l, err := net.ListenUDP("udp", addr)
	assert.NoError(t, err)
	f, err := l.File()
	assert.NoError(t, err)
	assert.NoError(t, l.Close())
	fd, err := syscall.Dup(int(f.Fd()))
	assert.NoError(t, err)
	assert.NoError(t, f.Close())
	return fd
}

func newFile(t *testing.T) int {
	f, err := ioutil.TempFile("", "newFile-")
	assert.NoError(t, err)
	fd, err := syscall.Dup(int(f.Fd()))
	assert.NoError(t, err)
	assert.NoError(t, f.Close())
	return fd
}

func CloseFd(t *testing.T, fd int) {
	f := os.NewFile(uintptr(fd), "")
	assert.NoError(t, f.Close())
}
