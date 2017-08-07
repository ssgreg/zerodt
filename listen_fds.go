// +build linux darwin

package zerodt

import (
	"fmt"
	"os"
	"strconv"
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

// listenFdsCount returns how many file descriptors have been passed.
func listenFdsCount() (count int, err error) {
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
	if countStr == "" {
		err = fmt.Errorf("mandatory environment variable does not exist: %s", envListenFds)
		return
	}
	count, err = strconv.Atoi(countStr)
	if err != nil {
		err = fmt.Errorf("bad environment variable: %s=%s", envListenFds, countStr)
		return
	}
	if count < 0 {
		err = fmt.Errorf("bad environment variable: %s=%d", envListenFds, count)
		return
	}
	return count, nil
}

// listenFds returns all inherited file descriptors.
func listenFds() ([]int, error) {
	count, err := listenFdsCount()
	if err != nil {
		return nil, err
	}
	fds := make([]int, count)
	for i := 0; i < count; i++ {
		fds[i] = listenFdsStart + i
	}
	return fds, nil
}

func prepareEnv(count int) *[]string {
	env := os.Environ()
	env = append(env, fmt.Sprintf("%s=%d", envListenFds, count))
	env = append(env, fmt.Sprintf("%s=%d", envListenPid, listenPidDefault))
	return &env
}

func unsetEnvAll() {
	// Ignore Unsetenv errors.
	os.Unsetenv(envListenPid)
	os.Unsetenv(envListenFds)
}
