package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"dangernoodle.io/terranoodle/internal/output"
)

// Version is set via ldflags at build time.
var Version string

var rootCmd = &cobra.Command{
	Use:          "terranoodle",
	Short:        "Unified Terragrunt/Terraform toolchain",
	Long:         "Unified Terragrunt/Terraform toolchain for catalog generation, state imports, and linting.",
	SilenceUsage: true,
}

var versionFlag bool

func init() {
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable color output")
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Print version and exit")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		noColor, _ := cmd.Flags().GetBool("no-color")
		if noColor || os.Getenv("NO_COLOR") != "" {
			output.Disable()
		}
		return nil
	}

	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if versionFlag {
			if Version != "" {
				fmt.Println(Version)
			} else {
				fmt.Println("(development build)")
			}
			return nil
		}
		return cmd.Help()
	}

	rootCmd.AddCommand(catalogCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(lintCmd)
	rootCmd.AddCommand(stateCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
