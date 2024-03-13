package cli

import (
	"context"

	"github.com/spf13/cobra"
	"go.keploy.io/server/v2/config"
)

type ServiceFactory interface {
	GetService(ctx context.Context, cmd string, config config.Config) (interface{}, error)
}

type CmdConfigurator interface {
	AddFlags(cmd *cobra.Command, config *config.Config) error
	ValidateFlags(ctx context.Context, cmd *cobra.Command, config *config.Config) error
}
