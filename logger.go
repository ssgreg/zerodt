// Copyright 2017 Grigory Zubankov. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.
//

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
}

var (
	// logger discards all log messages by default
	logger StdLogger = log.New(ioutil.Discard, "", 0)
)

type prefixLogger struct {
	StdLogger
	prefix string
}

func (l *prefixLogger) Print(args ...interface{}) {
	args = append([]interface{}{l.prefix}, args...)
	l.StdLogger.Print(args...)
}

func (l *prefixLogger) Printf(format string, args ...interface{}) {
	l.StdLogger.Printf(l.prefix+" "+format, args...)
}

func (l *prefixLogger) Println(args ...interface{}) {
	args = append([]interface{}{l.prefix}, args...)
	l.StdLogger.Println(args...)
}

// SetLogger allows to set a different logger that is compatible with StdLogger
// interface. Tested with stdlib logger:
//
//   log.New(os.Stderr, "", log.LstdFlags)
//
// And sirupsen/logrus:
//
//   logrus.StandardLogger()
//
func SetLogger(l StdLogger) {
	logger = &prefixLogger{StdLogger: l, prefix: "zerodt:"}
}
