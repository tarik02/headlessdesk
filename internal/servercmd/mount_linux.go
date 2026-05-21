//go:build linux

package servercmd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"headlessdesk/internal/control"
	"headlessdesk/internal/fusefs"
)

func newMountCommand(v *viper.Viper, configPath *string) *cobra.Command {
	var debug bool

	cmd := &cobra.Command{
		Use:   "mount [MOUNTPOINT]",
		Short: "Mount desktop screenshot, health, and input control files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(v, *configPath)
			if err != nil {
				return err
			}
			applyChangedFlags(cmd, &cfg)
			if err := resolveBackendExtends(&cfg); err != nil {
				return err
			}
			if err := validateConfig(cfg); err != nil {
				return err
			}
			mountpoint, err := resolveMountpoint(args)
			if err != nil {
				return err
			}
			return runFuseMount(cfg, mountpoint, fusefs.Options{Debug: debug})
		},
	}

	cmd.Flags().BoolVar(&debug, "debug", false, "enable FUSE debug logging")
	return cmd
}

func runFuseMount(cfg config, mountpoint string, options fusefs.Options) error {
	if err := os.MkdirAll(mountpoint, 0700); err != nil {
		return fmt.Errorf("create mountpoint: %w", err)
	}

	backends, err := startBackends(cfg)
	if err != nil {
		return err
	}
	defer closeComponent(backends.component)
	logComponentEnd("backend graph", backends.component)

	service := control.NewService(backends.component, backends.output, backends.input)
	server, err := fusefs.New(service, options).Mount(mountpoint)
	if err != nil {
		return fmt.Errorf("mount fuse filesystem: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("fuse filesystem mounted on %s", mountpoint)
	serverDone := make(chan struct{})
	go func() {
		server.Wait()
		close(serverDone)
	}()

	select {
	case <-ctx.Done():
		return server.Unmount()
	case <-backends.component.Done():
		err := backends.component.Err()
		if unmountErr := server.Unmount(); unmountErr != nil {
			return errors.Join(err, unmountErr)
		}
		return err
	case <-serverDone:
		return nil
	}
}
