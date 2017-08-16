// +build linux darwin

package zerodt

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// PipeMessenger holds files needed to perform interprocess communications.
//
// To make a channel you need a reader and a writer from different calls
// of os.Pipe(), e.g.:
//
//   // child pipe
//   cr, cw, _ := os.Pipe()
//   // parent pipe
//   pr, pw, _ := os.Pipe()
//
//   // child pipe messenger
//   cpm := NewPipeMessenger(cr, pw)
//   // parent pipe messenger
//   ppm := NewPipeMessenger(pr, cw)
//
// Note! PipeMessenger is not threadsafe. Take care of it if you need.
//
// Packet format:
// +-----------------------+---------+
// |   Header (8 bytes)    | Payload |
// +-----------------------+---------+
// | MagicN | Payload Size | Payload |
// +-----------------------+---------+
//
type PipeMessenger struct {
	r *os.File
	w *os.File
}

// NewPipeMessenger makes a new object
func NewPipeMessenger(r *os.File, w *os.File) *PipeMessenger {
	return &PipeMessenger{r: r, w: w}
}

// Recv receives a message from the channel synchronously.
func (p *PipeMessenger) Recv() ([]byte, error) {
	h := header{}
	err := binary.Read(p.r, binary.LittleEndian, &h)
	if err != nil {
		return nil, err
	}
	// Read the whole message to avoid breaking the stream.
	rs := make([]byte, h.Size)
	_, err = io.ReadFull(p.r, rs)
	if err != nil {
		return nil, err
	}
	return rs, nil
}

// Send sends a message to the channel synchronously.
func (p *PipeMessenger) Send(data []byte) error {
	err := binary.Write(p.w, binary.LittleEndian, newHeader(len(data)))
	if err != nil {
		return err
	}
	_, err = p.w.Write(data)
	if err != nil {
		return err
	}
	return nil
}

// Close closes both os.File
func (p *PipeMessenger) Close() error {
	err1 := p.r.Close()
	err2 := p.w.Close()
	if err1 != nil && err2 != nil {
		return fmt.Errorf("reader error: '%v', writer error: '%v'", err1, err2)
	}
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
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
