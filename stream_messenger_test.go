// Copyright 2017 Grigory Zubankov. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
//
// +build linux darwin

package zerodt

import (
	"net"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testMsg struct {
	Int    int
	String string
	Binary [35000]byte
}

func TestPipeMessenger(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)
	f0 := os.NewFile(uintptr(fds[0]), "s|0")
	f1 := os.NewFile(uintptr(fds[1]), "s|1")

	m0, err := ListenSocket(f0)
	require.NoError(t, err)
	defer func() { require.NoError(t, m0.Close()) }()
	m1, err := ListenSocket(f1)
	require.NoError(t, err)
	defer func() { require.NoError(t, m1.Close()) }()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		msg1 := testMsg{Int: 66, String: "Piped JSON"}
		msg1.Binary[0] = 42
		msg1.Binary[len(msg1.Binary)-1] = 43
		require.NoError(t, m0.Send(msg1))

		msg2 := testMsg{}
		require.NoError(t, m0.Recv(&msg2))
		assert.EqualValues(t, 77, msg2.Int)
		assert.Equal(t, "Piped JSON from Client", msg2.String)
		assert.EqualValues(t, 82, msg2.Binary[0])
		assert.EqualValues(t, 83, msg2.Binary[len(msg2.Binary)-1])

		msg3 := testMsg{Int: 166, String: "Piped JSON again"}
		msg3.Binary[0] = 142
		msg3.Binary[len(msg3.Binary)-1] = 143
		require.NoError(t, m0.Send(msg3))

		msg4 := testMsg{}
		require.NoError(t, m0.Recv(&msg4))
		assert.EqualValues(t, 177, msg4.Int)
		assert.Equal(t, "Piped JSON again from Client", msg4.String)
		assert.EqualValues(t, 182, msg4.Binary[0])
		assert.EqualValues(t, 183, msg4.Binary[len(msg2.Binary)-1])
	}()

	go func() {
		defer wg.Done()

		msg1 := testMsg{}
		require.NoError(t, m1.Recv(&msg1))
		assert.EqualValues(t, 66, msg1.Int)
		assert.Equal(t, "Piped JSON", msg1.String)
		assert.EqualValues(t, 42, msg1.Binary[0])
		assert.EqualValues(t, 43, msg1.Binary[len(msg1.Binary)-1])

		msg2 := testMsg{Int: 77, String: "Piped JSON from Client"}
		msg2.Binary[0] = 82
		msg2.Binary[len(msg2.Binary)-1] = 83
		require.NoError(t, m1.Send(msg2))

		msg3 := testMsg{}
		require.NoError(t, m1.Recv(&msg3))
		assert.EqualValues(t, 166, msg3.Int)
		assert.Equal(t, "Piped JSON again", msg3.String)
		assert.EqualValues(t, 142, msg3.Binary[0])
		assert.EqualValues(t, 143, msg3.Binary[len(msg3.Binary)-1])

		msg4 := testMsg{Int: 177, String: "Piped JSON again from Client"}
		msg4.Binary[0] = 182
		msg4.Binary[len(msg4.Binary)-1] = 183
		require.NoError(t, m1.Send(msg4))
	}()

	wg.Wait()
}

func TestPipeJSONMessengerWithDeadlinePartlySuccess(t *testing.T) {
	// workaround for darwin, something bad happens is file descriptors are reuses
	fdsTemp, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)
	defer syscall.Close(fdsTemp[0])
	defer syscall.Close(fdsTemp[1])
	// end workaround

	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)
	f0 := os.NewFile(uintptr(fds[0]), "s|0")
	f1 := os.NewFile(uintptr(fds[1]), "s|1")

	m0, err := ListenSocket(f0)
	require.NoError(t, err)
	defer func() { require.NoError(t, m0.Close()) }()
	m1, err := ListenSocket(f1)
	require.NoError(t, err)
	defer func() { require.NoError(t, m1.Close()) }()

	m0.SetDeadline(time.Now().Add(time.Millisecond * 2000))
	m1.SetDeadline(time.Now().Add(time.Millisecond * 2000))

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		msg1 := testMsg{Int: 66, String: "Piped JSON"}
		msg1.Binary[0] = 42
		msg1.Binary[len(msg1.Binary)-1] = 43
		require.NoError(t, m0.Send(msg1))

		m0.Send(msg1)
	}()

	go func() {
		defer wg.Done()

		time.Sleep(time.Millisecond * 1000)
		msg1 := testMsg{}
		require.NoError(t, m1.Recv(&msg1))
		assert.EqualValues(t, 66, msg1.Int)
		assert.Equal(t, "Piped JSON", msg1.String)
		assert.EqualValues(t, 42, msg1.Binary[0])
		assert.EqualValues(t, 43, msg1.Binary[len(msg1.Binary)-1])

		time.Sleep(time.Millisecond * 1000)
		require.Equal(t, true, m1.Recv(msg1).(*net.OpError).Timeout())
	}()

	wg.Wait()
}

func TestPipeJSONMessengerWithDeadlineFailWithRead(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)
	f0 := os.NewFile(uintptr(fds[0]), "s|0")

	m0, err := ListenSocket(f0)
	require.NoError(t, err)

	m0.SetReadDeadline(time.Now().Add(time.Millisecond * 300))

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		msg1 := testMsg{}
		require.Equal(t, true, m0.Recv(msg1).(*net.OpError).Timeout())
	}()

	wg.Wait()
}
