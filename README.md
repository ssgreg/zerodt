# ZeroDT [![Build Status](https://travis-ci.org/ssgreg/zerodt.svg?branch=master)](https://travis-ci.org/ssgreg/zerodt)

Zero downtime restart and graceful shutdown in one line of code

The package uses signals: syscall.SIGTERM, syscall.SIGINT, syscall.SIGUSR2