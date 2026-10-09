//go:build !windows

package winaudio

import "errors"

const (
	RoleConsole        = 0
	RoleMultimedia     = 1
	RoleCommunications = 2
)

func SetDefaultEndpoint(deviceID string, roles ...int) error {
	return errors.New("only available on Windows")
}
