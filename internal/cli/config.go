package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"dangernoodle.io/terranoodle/internal/config"
	"dangernoodle.io/terranoodle/internal/output"
)

var configGlobalFlag bool

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage terranoodle configuration",
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show effective configuration",
	RunE:  runConfigList,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a .terranoodle.yml with defaults",
	RunE:  runConfigInit,
}

func init() {
	configSetCmd.Flags().BoolVar(&configGlobalFlag, "global", false, "Set in global config")
	configInitCmd.Flags().BoolVar(&configGlobalFlag, "global", false, "Create global config")

	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configInitCmd)
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("config get: %w", err)
	}

	cfg, err := config.Discover(cwd)
	if err != nil {
		return err
	}

	val, err := cfg.Get(args[0])
	if err != nil {
		return err
	}

	fmt.Println(val)
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]

	var path string
	if configGlobalFlag {
		var err error
		path, err = config.GlobalPath()
		if err != nil {
			return err
		}
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("config set: %w", err)
		}
		path = filepath.Join(cwd, config.ProjectFile)
	}

	// Load existing or start empty
	cfg, err := config.Load(path)
	if err != nil {
		cfg = &config.Config{}
	}

	if err := cfg.Set(key, value); err != nil {
		return err
	}

	if err := config.Save(path, cfg); err != nil {
		return err
	}

	output.Success("Set %s = %s", key, value)
	return nil
}

func runConfigList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("config list: %w", err)
	}

	cfg, err := config.Discover(cwd)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config list: marshal: %w", err)
	}

	fmt.Print(string(data))
	return nil
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	var path string
	var label string

	if configGlobalFlag {
		var err error
		path, err = config.GlobalPath()
		if err != nil {
			return err
		}
		label = path
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("config init: %w", err)
		}
		path = filepath.Join(cwd, config.ProjectFile)
		label = config.ProjectFile
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config init: %s already exists", label)
	}

	if err := config.Save(path, config.Default()); err != nil {
		return err
	}

	output.Success("Created %s", label)
	return nil
}
