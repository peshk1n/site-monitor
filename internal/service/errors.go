package service

import "errors"

var (
	ErrMonitorNotFound = errors.New("monitor not found")
	ErrURLRequired     = errors.New("url is required")
	ErrNoChecksFound   = errors.New("no checks found")
)
