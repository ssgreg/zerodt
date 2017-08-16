// +build linux darwin

package zerodt

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestPipeMessenger(t *testing.T) {
	r1, w1, err := os.Pipe()
	require.NoError(t, err)
	r2, w2, err := os.Pipe()
	require.NoError(t, err)
	m := NewPipeMessenger(r1, w2)
	defer func() { require.NoError(t, m.Close()) }()
	m1 := NewPipeMessenger(r2, w1)
	defer func() { require.NoError(t, m1.Close()) }()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		msg1 := make([]byte, 100000)
		msg1[0] = 42
		msg1[len(msg1)-1] = 43
		require.NoError(t, m.Send(msg1))

		msg2, err := m.Recv()
		require.NoError(t, err)
		assert.Equal(t, 200000, len(msg2))
		assert.EqualValues(t, 82, msg2[0])
		assert.EqualValues(t, 83, msg2[len(msg2)-1])

		msg3 := make([]byte, 3)
		msg3[0] = 142
		msg3[len(msg3)-1] = 143
		require.NoError(t, m.Send(msg3))

		msg4, err := m.Recv()
		require.NoError(t, err)
		assert.Equal(t, 1, len(msg4))
		assert.EqualValues(t, 182, msg4[0])
	}()

	go func() {
		defer wg.Done()

		msg1, err := m1.Recv()
		require.NoError(t, err)
		assert.Equal(t, 100000, len(msg1))
		assert.EqualValues(t, 42, msg1[0])
		assert.EqualValues(t, 43, msg1[len(msg1)-1])

		msg2 := make([]byte, 200000)
		msg2[0] = 82
		msg2[len(msg2)-1] = 83
		require.NoError(t, m1.Send(msg2))

		msg3, err := m1.Recv()
		require.NoError(t, err)
		assert.Equal(t, 3, len(msg3))
		assert.EqualValues(t, 142, msg3[0])
		assert.EqualValues(t, 143, msg3[len(msg3)-1])

		msg4 := make([]byte, 1)
		msg4[0] = 182
		require.NoError(t, m1.Send(msg4))
	}()

	wg.Wait()
}
