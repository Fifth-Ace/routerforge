//go:build windows

package main

func readPlatformStorage(mount string) platformStorage {
	return platformStorage{Mount: mount}
}
