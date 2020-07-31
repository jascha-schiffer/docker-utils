package service

import (
	"github.com/spf13/cobra"

	"github.com/docker/cli/cli"
	"github.com/docker/cli/cli/command"
)

// NewServiceCommand returns a cobra command for `service` subcommands
func NewServiceCommand(dockerCli command.Cli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Services utilities",
		Args:  cli.NoArgs,
		RunE:  command.ShowHelp(dockerCli.Err()),
	}
	cmd.AddCommand(
		newWaitCommand(dockerCli),
	)
	return cmd
}
