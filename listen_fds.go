// +build linux darwin

package zerodt

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"syscall"
)

const (
	// systemd's socket activation environment variables:
	// Number of provided descriptors.
	envListenFds = "LISTEN_FDS"
	// Who should handle socket activation.
	envListenPid = "LISTEN_PID"

	// The first passed file descriptor is fd 3.
	listenFdsStart = 3
	// There is no easy way in golang to do separate fork/exec in
	// order to know child pid. Use this constant instead.
	listenPidDefault = 0
)

// Set socket options, to make sure a socket is the same as golang
// http library provides.
//
// E.g. systemd's unit file does not set these options
// by default: SO_BROADCAST, SO_REUSEADDR, TCP_NODELAY.
//
// It's also important to switch a socket to a non-blocking mode, but
// net.FileListener do this for us.
func setDefaultGoSocketOptions(fd int) error {
	// Allow broadcast:
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1); err != nil {
		return err
	}
	// Allow reuse of recently-used addresses:
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return err
	}
	// Allow sending packets immediately
	if err := syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1); err != nil {
		return err
	}
	return nil
}

// unsetEnvAll is helper function to unset all passed environment variables.
func unsetEnvAll(unsetEnv bool) {
	if !unsetEnv {
		return
	}
	// Ignore Unsetenv errors.
	os.Unsetenv(envListenPid)
	os.Unsetenv(envListenFds)
}

// listenFdsCount returns how many file descriptors have been passed.
func listenFdsCount(unsetEnv bool) (count int, err error) {
	pidStr := os.Getenv(envListenPid)
	// Normal exit - nothing to listen.
	if pidStr == "" {
		return
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		err = fmt.Errorf("bad environment variable: %s=%s", envListenPid, pidStr)
		return
	}
	// Is this for us?
	if pid != listenPidDefault && pid != os.Getpid() {
		err = fmt.Errorf("bad environment variable: %s=%d with pid=%d", envListenPid, pid, os.Getpid())
		return
	}
	countStr := os.Getenv(envListenFds)
	if pidStr == "" {
		err = fmt.Errorf("mandatory environment variable does not exist: %s", envListenFds)
		return
	}
	count, err = strconv.Atoi(countStr)
	if err != nil {
		err = fmt.Errorf("bad environment variable: %s=%s", envListenFds, pidStr)
		return
	}
	if count < 0 {
		err = fmt.Errorf("bad environment variable: %s=%d", envListenFds, count)
	}

	unsetEnvAll(unsetEnv)
	return count, nil
}

// listenFds returns all passed file descriptors.
func listenFds() ([]int, error) {
	count, err := listenFdsCount(true)
	if err != nil {
		return nil, err
	}
	fds := make([]int, count)
	for i := 0; i < count; i++ {
		fds[i] = listenFdsStart + i
	}
	return fds, nil
}

// isSocketTCP checks if passed file descriptor is a TCP socket
func isSocketTCP(fd int) (bool, error) {
	// check S_IFSOCK flag
	var st syscall.Stat_t
	err := syscall.Fstat(fd, &st)
	if err != nil {
		return false, err
	}
	switch st.Mode & syscall.S_IFMT {
	case syscall.S_IFSOCK:
	default:
		return false, nil
	}
	// check SOCK_STREAM option
	socketType, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_TYPE)
	if err != nil {
		return false, err
	}
	if socketType != syscall.SOCK_STREAM {
		return false, nil
	}
	return true, nil
}

// makeSocketFilename makes a filename the same way as golang does.
func makeSocketFilename(fd int) (string, error) {
	lsa, err := syscall.Getsockname(fd)
	if err != nil {
		return "", err
	}

	var name string
	switch sa := lsa.(type) {
	case *syscall.SockaddrInet4:
		addr := net.TCPAddr{IP: sa.Addr[0:], Port: sa.Port}
		name = addr.String()
	case *syscall.SockaddrInet6:
		// Use zone id instead of zone name, unfortunately `zoneToString` is private in golang.
		addr := net.TCPAddr{IP: sa.Addr[0:], Port: sa.Port, Zone: fmt.Sprintf("%v", sa.ZoneId)}
		name = addr.String()
	default:
		return "", fmt.Errorf("unsupported sockaddr type")
	}

	return name, nil
}

// newFileOnSocket turns file descriptor (only TCP sockets allowed) into *os.File
func newFileOnSocket(fd int) (*os.File, error) {
	// Check if a passed file descriptor is a TCP socket. Other fd types
	// shall not pass.
	ok, err := isSocketTCP(fd)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("newFile: file descriptor `%v` is not a socket", fd)
	}

	// Make sure all needed options have been set.
	err = setDefaultGoSocketOptions(fd)
	if err != nil {
		return nil, err
	}

	// To tell to truth filename is optional for sockets. But we need
	// to do out best.
	name, err := makeSocketFilename(fd)
	if err != nil {
		return nil, err
	}

	return os.NewFile(uintptr(fd), name), nil
}

// newFileTCPListener returns a copy of the network TCP listener
// corresponding to the open file f.
// It is the caller's responsibility to close ln when finished.
func newFileTCPListener(f *os.File) (*net.TCPListener, error) {
	l, err := net.FileListener(f)
	if err != nil {
		return nil, err
	}
	switch tl := l.(type) {
	case *net.TCPListener:
		return tl, nil
	default:
		// There is no way to get here. Nevertheless if it can happen,
		// it will happen.
		return nil, fmt.Errorf("file is not a TCP socket")
	}
}
