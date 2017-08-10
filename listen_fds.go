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
	envListenFDS = "LISTEN_FDS"
	// Who should handle socket activation.
	envListenPID = "LISTEN_PID"

	// The first passed file descriptor is fd 3.
	listenFDSStart = 3
	// There is no easy way in golang to do separate fork/exec in
	// order to know child pid. Use this constant instead.
	listenPIDDefault = 0
)

// listenFdsCount returns how many file descriptors have been passed.
func listenFdsCount() (count int, err error) {
	pidStr := os.Getenv(envListenPID)
	// Normal exit - nothing to listen.
	if pidStr == "" {
		return
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		err = fmt.Errorf("bad environment variable: %s=%s", envListenPID, pidStr)
		return
	}
	// Is this for us?
	if pid != listenPIDDefault && pid != os.Getpid() {
		err = fmt.Errorf("bad environment variable: %s=%d with pid=%d", envListenPID, pid, os.Getpid())
		return
	}
	countStr := os.Getenv(envListenFDS)
	if countStr == "" {
		err = fmt.Errorf("mandatory environment variable does not exist: %s", envListenFDS)
		return
	}
	count, err = strconv.Atoi(countStr)
	if err != nil {
		err = fmt.Errorf("bad environment variable: %s=%s", envListenFDS, countStr)
		return
	}
	if count < 0 {
		err = fmt.Errorf("bad environment variable: %s=%d", envListenFDS, count)
		return
	}
	return
}

// listenFds returns all inherited file descriptors.
func listenFds() ([]int, error) {
	count, err := listenFdsCount()
	if err != nil {
		return nil, err
	}
	fds := make([]int, count)
	for i := 0; i < count; i++ {
		fds[i] = listenFDSStart + i
	}
	return fds, nil
}

func prepareEnv(count int) []string {
	env := os.Environ()
	env = append(env, fmt.Sprintf("%s=%d", envListenFDS, count))
	env = append(env, fmt.Sprintf("%s=%d", envListenPID, listenPIDDefault))
	return env
}

func unsetEnvAll() {
	// Ignore Unsetenv errors.
	os.Unsetenv(envListenPID)
	os.Unsetenv(envListenFDS)
}
