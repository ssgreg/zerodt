// Copyright 2017 Grigory Zubankov. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
//
// +build linux darwin

package zerodt

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"time"
)

// StreamMessenger a simple messenger based on net.Conn.
// The simplest way to create messenger is to use syscall.Socketpair():
//
//	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
//	f0 := os.NewFile(uintptr(fds[0]), "s|0")
//	f1 := os.NewFile(uintptr(fds[1]), "s|1")
//
//	m0, err := ListenSocket(f0)
//	m1, err := ListenSocket(f1)
//
//
// Packet format:
// +-----------------------+---------+
// |   Header (8 bytes)    | Payload |
// +-----------------------+---------+
// | MagicN | Payload Size | Payload |
// +-----------------------+---------+
//
type StreamMessenger struct {
	c net.Conn
}

// ListenSocket TODO
func ListenSocket(s *os.File) (*StreamMessenger, error) {
	defer s.Close()
	c, err := net.FileConn(s)
	if err != nil {
		return nil, err
	}
	return &StreamMessenger{c}, nil
}

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail with a timeout (see type Error) instead of
// blocking. The deadline applies to all future and pending
// I/O, not just the immediately following call to Read or
// Write. After a deadline has been exceeded, the connection
// can be refreshed by setting a deadline in the future.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (m *StreamMessenger) SetDeadline(t time.Time) error {
	return m.c.SetDeadline(t)
}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (m *StreamMessenger) SetReadDeadline(t time.Time) error {
	return m.c.SetReadDeadline(t)
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (m *StreamMessenger) SetWriteDeadline(t time.Time) error {
	return m.c.SetWriteDeadline(t)
}

// Recv receives a message from the channel.
func (m *StreamMessenger) Recv(v interface{}) (err error) {
	b, err := m.recv()
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// Send sends a message to the channel.
func (m *StreamMessenger) Send(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return m.send(b)
}

func (m *StreamMessenger) recv() ([]byte, error) {
	h := header{}
	err := binary.Read(m.c, binary.LittleEndian, &h)
	if err != nil {
		return nil, err
	}
	if !isValidHeader(h) {
		return nil, errors.New("StreamMessenger: the header is invalid")
	}
	// Read the whole message to avoid breaking the stream.
	rs := make([]byte, h.Size)
	_, err = io.ReadFull(m.c, rs)
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func (m *StreamMessenger) send(data []byte) error {
	err := binary.Write(m.c, binary.LittleEndian, newHeader(len(data)))
	if err != nil {
		return err
	}
	_, err = m.c.Write(data)
	if err != nil {
		return err
	}
	return nil
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (m *StreamMessenger) Close() error {
	return m.c.Close()
}

const (
	headerPrefix = uint32(0x5a45524f)
)

type header struct {
	Prefix uint32
	Size   uint32
}

func newHeader(size int) header {
	return header{headerPrefix, uint32(size)}
}

func isValidHeader(h header) bool {
	return h.Prefix == headerPrefix
}
