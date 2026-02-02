//go:build linux || android

package utils

import "syscall"

func Sync() {
	syscall.Sync()
}
