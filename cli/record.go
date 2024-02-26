package cli

import (
	"context"

	"github.com/spf13/cobra"
	"go.keploy.io/server/v2/config"
	"go.keploy.io/server/v2/pkg/models"
	"go.uber.org/zap"
)

var filters = models.TestFilter{}

func init() {
	Register("record", Record)
}

func Record(ctx context.Context, logger *zap.Logger, cfg *config.Config, serviceFactory ServiceFactory, cmdConfigurator CmdConfigurator) *cobra.Command {
	var cmd = &cobra.Command{
		Use:     "record",
		Short:   "record the keploy testcases from the API calls",
		Example: `keploy record -c "/path/to/user/app"`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return cmdConfigurator.ValidateFlags(cmd, cfg)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			recorder.record()
			return nil
		},
	}

	cmdConfigurator.AddFlags(cmd, cfg)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd
}
