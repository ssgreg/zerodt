// +build linux darwin

package zerodt

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"
)

// TODO: use non-blocking file operations that introduced in go1.9

// Errors returned by PipeJSONMessenger.
var (
	ErrTimeout = errors.New("PipeJSONMessenger: timeout")
)

// PipeJSONMessenger based of a PipeMessenger.
//
// Provides two additional benefits:
// - JSON format to marshal/unmarshal messages
// - Deadline to control I/O operations timeout
type PipeJSONMessenger struct {
	m *PipeMessenger
	t time.Time
}

// ListenPipe simplifies object creation
func ListenPipe(r *os.File, w *os.File) *PipeJSONMessenger {
	return NewPipeJSONMessenger(NewPipeMessenger(r, w))
}

// NewPipeJSONMessenger makes a new object
func NewPipeJSONMessenger(m *PipeMessenger) *PipeJSONMessenger {
	return &PipeJSONMessenger{m, time.Time{}}
}

// Recv receives a message from the channel synchronously.
func (p *PipeJSONMessenger) Recv(v interface{}) (err error) {
	return p.ioWithDeadline(func(v interface{}) error {
		b, err := p.m.Recv()
		if err != nil {
			return err
		}
		return json.Unmarshal(b, v)
	}, v)
}

// Send sends a message to the channel synchronously.
func (p *PipeJSONMessenger) Send(v interface{}) error {
	return p.ioWithDeadline(func(v interface{}) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return p.m.Send(b)
	}, v)
}

// Close closes underlying PipeMessenger
func (p *PipeJSONMessenger) Close() error {
	return p.m.Close()
}

// SetDeadline allows to set a deadline for I/O operations.
// A zero value for t means Read and Write will not time out.
// Default value if zero.
func (p *PipeJSONMessenger) SetDeadline(t time.Time) error {
	p.t = t
	return nil
}

func (p *PipeJSONMessenger) ioWithDeadline(fn func(v interface{}) error, v interface{}) (err error) {
	var wg sync.WaitGroup
	wg.Add(1)
	// A bufferred channel to avoid deadlock.
	done := make(chan bool, 1)

	go func() {
		defer wg.Done()
		err = fn(v)
		done <- true
	}()

	var chTime <-chan time.Time
	if !p.t.IsZero() {
		chTime = time.NewTimer(p.t.Sub(time.Now())).C
	}

	// Wait for a 'done' channel or a deadline channel.
	timeout := false
	select {
	case <-done:
	case <-chTime:
		// There is no way to cancel blocking IO, except closing it.
		p.Close()
		timeout = true
	}
	wg.Wait()

	// Fix error in case of timeout. Do not change 'err' before wg.Wait()
	// because of a race condition.
	if timeout {
		err = ErrTimeout
	}
	return
}
