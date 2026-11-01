//go:build !linux

package lite

import (
	"context"
	"errors"
)

func AcquireProcessLock(string) (func(), error) {
	return nil, errors.New("DNS controller supervision requires Linux")
}
func RecordSocketOwner(string) error  { return errors.New("DNS controller supervision requires Linux") }
func RecoverOwnedSocket(string) error { return errors.New("DNS controller supervision requires Linux") }
func RunSupervised(context.Context, string, string, string) error {
	return errors.New("DNS controller supervision requires Linux")
}
