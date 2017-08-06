package zerodt

import (
	"io/ioutil"
	"log"
)

// StdLogger is an interface for stdlib logger also compatible with logrus
type StdLogger interface {
	Print(...interface{})
	Printf(string, ...interface{})
	Println(...interface{})

	Fatal(...interface{})
	Fatalf(string, ...interface{})
	Fatalln(...interface{})

	Panic(...interface{})
	Panicf(string, ...interface{})
	Panicln(...interface{})
}

var (
	// logger discards all log messages by default
	logger StdLogger = log.New(ioutil.Discard, "", 0)
)

// SetLogger allows to set a different logger that compatible with StdLogger
// interface. Tested with stdlib logger:
//
//   log.New(os.Stderr, "", log.LstdFlags)
//
// And sirupsen/logrus:
//
//   logrus.StandardLogger()
//
func SetLogger(l StdLogger) {
	logger = l
}
