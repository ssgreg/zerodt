// +build linux darwin

package zerodt

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrepareEnv(t *testing.T) {
	os.Setenv("TEST_PREPARE_ENV", "EXISTS")
	env := prepareEnv(7)
	if len(*env) < 3 {
		t.Fail()
	}
	assert.NotEmpty(t, isExists(env, "LISTEN_FDS=7"))
	assert.NotEmpty(t, isExists(env, "LISTEN_PID=0"))
	assert.NotEmpty(t, isExists(env, "TEST_PREPARE_ENV=EXISTS"))
}

func TestUnsetEnvAll(t *testing.T) {
	os.Setenv("LISTEN_FDS", "7")
	os.Setenv("LISTEN_PID", "0")

	unsetEnvAll()
	assert.Equal(t, "", os.Getenv("LISTEN_FDS"))
	assert.Equal(t, "", os.Getenv("LISTEN_PID"))
}

func TestListenFdsCount(t *testing.T) {
	// Normal exit without activation
	setEnv("", "")
	count, err := listenFdsCount()
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	// Bad LISTEN_PID
	setEnv("not a pid", "")
	count, err = listenFdsCount()
	assertErr(t, err, "^bad environment variable: LISTEN_PID=not a pid$")

	setEnv("777", "")
	count, err = listenFdsCount()
	assertErr(t, err, fmt.Sprintf("^bad environment variable: LISTEN_PID=777 with pid=%d$", os.Getpid()))

	// No LISTEN_FDS
	setEnv(strconv.Itoa(os.Getpid()), "")
	count, err = listenFdsCount()
	assertErr(t, err, "^mandatory environment variable does not exist: LISTEN_FDS$")

	// Bad LISTEN_FDS
	setEnv(strconv.Itoa(os.Getpid()), "not a number")
	count, err = listenFdsCount()
	assertErr(t, err, fmt.Sprintf("^bad environment variable: LISTEN_FDS=not a number$"))

	setEnv(strconv.Itoa(os.Getpid()), "-2")
	count, err = listenFdsCount()
	assertErr(t, err, fmt.Sprintf("^bad environment variable: LISTEN_FDS=-2$"))

	// All ok
	setEnv(strconv.Itoa(os.Getpid()), "7")
	count, err = listenFdsCount()
	assert.NoError(t, err)
	assert.Equal(t, 7, count)

	// All ok with default LISTEN_PID
	setEnv("0", "148")
	count, err = listenFdsCount()
	assert.NoError(t, err)
	assert.Equal(t, 148, count)
}

func TestListenFds(t *testing.T) {
	// Normal exit without activation
	setEnv("", "")
	fds, err := listenFds()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(fds))

	// Normal exit with activation
	setEnv("0", "2")
	fds, err = listenFds()
	assert.NoError(t, err)
	assert.Equal(t, []int{3, 4}, fds)

	// Bad env
	setEnv("not a pid", "")
	_, err = listenFds()
	assert.Error(t, err)
}

func setEnv(pid, fds string) {
	unsetEnvAll()
	if pid != "" {
		os.Setenv(envListenPid, pid)
	}
	if fds != "" {
		os.Setenv(envListenFds, fds)
	}
}

func isExists(a *[]string, v string) bool {
	for _, s := range *a {
		if s == v {
			return true
		}
	}
	return false
}

func assertErr(t *testing.T, e error, rx interface{}) {
	assert.Error(t, e)
	assert.Regexp(t, rx, e.Error())
}
