// +build linux darwin

package zerodt

import (
	"fmt"
	"net"
	"os"
	"syscall"
)

// A Lyrical Digression.
//
// During the early alpha version of the package, I discovered an issue
// connected with TCPListener's File() function.
//
// Sometimes my demo http server hangs up after successful Shutdown call.
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
// To avoid the race condition I decided to duplicate a original listeners
// just before server.Serve() and put it into non-blocking mode by myself.
// This forces me to open and keep additional file descriptor per each
// listener, but it's worth it.

// fileListenerPair describes a pair of a TCPListener and a File.
type fileListenerPair struct {
	l *net.TCPListener
	f *os.File
}

// inherit returns all inherited listeners with
// duplicated file descriptors wrapped in os.File.
// Can be called only once.
func inherit() ([]*fileListenerPair, *StreamMessenger, error) {
	// Are there some listeners to inherit?
	fds, err := listenFds()
	if err != nil {
		return nil, nil, err
	}
	pairs, cp, err := inheritWithFDS(fds)
	if err != nil {
		return nil, nil, err
	}
	unsetEnvAll()
	return pairs, cp, nil
}

func inheritWithFDS(fds []int) ([]*fileListenerPair, *StreamMessenger, error) {
	m, err := getMessengerWithFDS(fds)
	if err != nil {
		return nil, nil, err
	}
	if m != nil {
		fds = fds[0 : len(fds)-1]
	}
	// Start to listen them.
	pairs := make([]*fileListenerPair, len(fds))
	for i, fd := range fds {
		f, err := newFileOnSocket(fd)
		if err != nil {
			return nil, nil, err
		}
		l, err := newFileTCPListener(f)
		if err != nil {
			return nil, nil, err
		}
		pairs[i] = &fileListenerPair{l, f}
	}
	return pairs, m, nil
}

func getMessengerWithFDS(fds []int) (*StreamMessenger, error) {
	count := len(fds)
	if count > 0 {
		ok, err := isSocketTCP(fds[count-1])
		if err != nil {
			return nil, err
		}
		if !ok {
			s := os.NewFile(uintptr(fds[count-1]), "s|0")
			m, err := ListenSocket(s)
			if err != nil {
				return nil, err
			}
			return m, nil
		}
	}
	return nil, nil
}

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

// isSocketTCP checks if passed file descriptor is a TCP socket
func isSocketTCP(fd int) (bool, error) {
	// Check S_IFSOCK flag.
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
	// Check SOCK_STREAM option.
	socketType, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_TYPE)
	if err != nil {
		return false, err
	}
	if socketType != syscall.SOCK_STREAM {
		return false, nil
	}
	// Check is it Unix socket.
	lsa, err := syscall.Getsockname(fd)
	if err != nil {
		return false, err
	}
	switch lsa.(type) {
	case *syscall.SockaddrUnix:
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
		addr := net.TCPAddr{IP: sa.Addr[0:], Port: sa.Port, Zone: fmt.Sprintf("%d", sa.ZoneId)}
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
	// to do our best.
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
