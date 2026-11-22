//go:build !linux

package lite

func newSystemConntrack() *ConntrackCleaner { return &ConntrackCleaner{} }
