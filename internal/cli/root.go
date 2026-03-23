package cli

import (
	"github.com/spf13/cobra"

	"dangernoodle.io/terra-tools/internal/output"
)

var rootCmd = &cobra.Command{
	Use:          "terra-tools",
	Short:        "Terragrunt/Terraform toolchain",
	SilenceUsage: true,
}

func init() {
	var noColor bool

	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable color output")

	cobra.OnInitialize(func() {
		if noColor {
			output.Disable()
		}
	})

	rootCmd.AddCommand(catalogCmd)
	rootCmd.AddCommand(lintCmd)
	rootCmd.AddCommand(stateCmd)
	rootCmd.AddCommand(versionCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		output.Error("%s", err)
	}
}
