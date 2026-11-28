//go:build !linux

package apptransaction

import "errors"

func acquireDevicePackageLease() (*DevicePackageLease, error) {
	return nil, errors.New("device package lease unsupported")
}
