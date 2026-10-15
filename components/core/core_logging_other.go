//go:build !linux

package main

func coreRAMLoggingAvailable() bool { return false }
