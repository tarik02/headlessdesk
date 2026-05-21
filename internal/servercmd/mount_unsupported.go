//go:build !linux

package servercmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newMountCommand(v *viper.Viper, configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "mount [MOUNTPOINT]",
		Short: "Mount desktop screenshot, health, and input control files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("mount command is only supported on linux")
		},
	}
}
