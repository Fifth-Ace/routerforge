//go:build linux

package apptransaction

func acquireDevicePackageLease() (*DevicePackageLease, error) {
	c, err := DeviceCoordinator()
	if err != nil {
		return nil, err
	}
	return acquireCoordinatorPackageLease(c)
}

func acquireCoordinatorPackageLease(c *Coordinator) (*DevicePackageLease, error) {
	f, err := c.AcquireLegacy()
	if err != nil {
		return nil, err
	}
	return &DevicePackageLease{file: f}, nil
}
