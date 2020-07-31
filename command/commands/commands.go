package commands

import (
	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"
	"jascha-schiffer/docker-utils/command/service"
)

// AddCommands adds all the commands from cli/command to the root command
func AddCommands(cmd *cobra.Command, dockerCli command.Cli) {
	cmd.AddCommand(
		service.NewServiceCommand(dockerCli),
	)
}
