package logger

import (
    "log"
    "os"
)

// Package-level loggers for info, error, and fatal messages.
var (
    Info  *log.Logger
    Error *log.Logger
    Fatal *log.Logger
)

func init() {
    Info = log.New(os.Stdout, "", log.Ldate|log.Ltime)
    Error = log.New(os.Stderr, "", log.Ldate|log.Ltime)
    Fatal = log.New(os.Stderr, "", log.Ldate|log.Ltime)
}

