// +build linux darwin

package zerodt

import (
	"os"
	"sync"
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

func TestPipeJSONMessenger(t *testing.T) {
	r1, w1, err := os.Pipe()
	require.NoError(t, err)
	r2, w2, err := os.Pipe()
	require.NoError(t, err)
	m := ListenPipe(r1, w2)
	defer func() { require.NoError(t, m.Close()) }()
	m1 := ListenPipe(r2, w1)
	defer func() { require.NoError(t, m1.Close()) }()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		msg1 := testMsg{Int: 66, String: "Piped JSON"}
		msg1.Binary[0] = 42
		msg1.Binary[len(msg1.Binary)-1] = 43
		require.NoError(t, m.Send(msg1))

		msg2 := testMsg{}
		require.NoError(t, m.Recv(&msg2))
		assert.EqualValues(t, 77, msg2.Int)
		assert.Equal(t, "Piped JSON from Client", msg2.String)
		assert.EqualValues(t, 82, msg2.Binary[0])
		assert.EqualValues(t, 83, msg2.Binary[len(msg2.Binary)-1])

		msg3 := testMsg{Int: 166, String: "Piped JSON again"}
		msg3.Binary[0] = 142
		msg3.Binary[len(msg3.Binary)-1] = 143
		require.NoError(t, m.Send(msg3))

		msg4 := testMsg{}
		require.NoError(t, m.Recv(&msg4))
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
	r1, w1, err := os.Pipe()
	require.NoError(t, err)
	r2, w2, err := os.Pipe()
	require.NoError(t, err)
	m := ListenPipe(r1, w2)
	defer func() { require.Error(t, m.Close()) }()
	m1 := ListenPipe(r2, w1)
	defer func() { require.Error(t, m1.Close()) }()

	m.SetDeadline(time.Now().Add(time.Millisecond * 400))
	m1.SetDeadline(time.Now().Add(time.Millisecond * 400))

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		msg1 := testMsg{Int: 66, String: "Piped JSON"}
		msg1.Binary[0] = 42
		msg1.Binary[len(msg1.Binary)-1] = 43
		require.NoError(t, m.Send(msg1))

		require.Equal(t, ErrTimeout, m.Send(msg1))
	}()

	go func() {
		defer wg.Done()

		time.Sleep(time.Millisecond * 200)
		msg1 := testMsg{}
		require.NoError(t, m1.Recv(&msg1))
		assert.EqualValues(t, 66, msg1.Int)
		assert.Equal(t, "Piped JSON", msg1.String)
		assert.EqualValues(t, 42, msg1.Binary[0])
		assert.EqualValues(t, 43, msg1.Binary[len(msg1.Binary)-1])

		time.Sleep(time.Millisecond * 200)
		require.Equal(t, ErrTimeout, m1.Recv(msg1))
	}()

	wg.Wait()
}

func TestPipeJSONMessengerWithDeadlineFailWithWriter(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	m := ListenPipe(r, w)
	defer func() { require.Error(t, m.Close()) }()

	m.SetDeadline(time.Now().Add(time.Millisecond * 300))

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		msg1 := testMsg{}
		require.Equal(t, ErrTimeout, m.Send(msg1))
	}()

	wg.Wait()
}

func TestPipeJSONMessengerWithDeadlineFailWithRead(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	m := ListenPipe(r, w)
	defer func() { require.Error(t, m.Close()) }()

	m.SetDeadline(time.Now().Add(time.Millisecond * 300))

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		msg1 := testMsg{}
		require.Equal(t, ErrTimeout, m.Recv(msg1))
	}()

	wg.Wait()
}
