package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"dangernoodle.io/terranoodle/internal/config"
	"dangernoodle.io/terranoodle/internal/output"
)

var configGlobalFlag bool
var configProfileFlag string
var configLongFlag bool

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
	configSetCmd.Flags().StringVar(&configProfileFlag, "profile", "", "Target profile (implies --global)")
	configGetCmd.Flags().StringVar(&configProfileFlag, "profile", "", "Read from a specific profile")
	configInitCmd.Flags().BoolVar(&configGlobalFlag, "global", false, "Create global config")
	configInitCmd.Flags().BoolVar(&configLongFlag, "long", false, "Generate annotated config with descriptions and options")

	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configInitCmd)
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	if configProfileFlag != "" {
		globalPath, err := config.GlobalPath()
		if err != nil {
			return err
		}

		globalCfg, err := config.LoadGlobal(globalPath)
		if err != nil {
			return fmt.Errorf("config get: load global config: %w", err)
		}

		profile, ok := globalCfg.Profiles[configProfileFlag]
		if !ok {
			return fmt.Errorf("config get: profile %q not found", configProfileFlag)
		}

		// Handle scaffold.* keys directly (not part of lint Config)
		key := args[0]
		switch key {
		case "scaffold.state":
			fmt.Println(profile.Scaffold.State)
			return nil
		case "scaffold.providers":
			fmt.Println(strings.Join(profile.Scaffold.Providers, ","))
			return nil
		}

		tempCfg := &config.Config{Lint: profile.Lint}
		val, err := tempCfg.Get(key)
		if err != nil {
			return err
		}

		fmt.Println(val)
		return nil
	}

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

	// --profile implies --global
	if configProfileFlag != "" {
		configGlobalFlag = true
	}

	if configGlobalFlag {
		var path string
		var err error
		path, err = config.GlobalPath()
		if err != nil {
			return err
		}

		// Load existing global config or create empty
		var globalCfg *config.GlobalConfig
		globalCfg, err = config.LoadGlobal(path)
		if err != nil {
			globalCfg = &config.GlobalConfig{Profiles: make(map[string]config.Profile)}
		}

		// Determine target profile name
		profileName := "default"
		if configProfileFlag != "" {
			profileName = configProfileFlag
		}

		// Get existing profile or create empty
		profile := globalCfg.Profiles[profileName]

		// Handle special "bind" key
		if key == "bind" {
			profile.Bind = strings.Split(value, ",")
		} else if strings.HasPrefix(key, "lint.") {
			// Handle lint.* keys
			if profile.Lint.Rules == nil {
				profile.Lint.Rules = make(map[string]config.RuleConfig)
			}
			tempCfg := &config.Config{Lint: profile.Lint}
			if err := tempCfg.Set(key, value); err != nil {
				return err
			}
			profile.Lint = tempCfg.Lint
		} else if key == "scaffold.state" {
			profile.Scaffold.State = value
		} else if key == "scaffold.providers" {
			profile.Scaffold.Providers = strings.Split(value, ",")
		} else {
			return fmt.Errorf("config set: unsupported key %q for profile", key)
		}

		// Write profile back to global config
		globalCfg.Profiles[profileName] = profile

		if err := config.SaveGlobal(path, globalCfg); err != nil {
			return err
		}

		output.Success("Set %s = %s", key, value)
		return nil
	}

	// Project config behavior (unchanged)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("config set: %w", err)
	}
	path := filepath.Join(cwd, config.ProjectFile)

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

	if configLongFlag {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("config init: %w", err)
		}
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("config init: %w", err)
		}
		defer f.Close()
		if err := config.RenderLong(f); err != nil {
			return fmt.Errorf("config init: %w", err)
		}
	} else if configGlobalFlag {
		if err := config.SaveGlobal(path, config.DefaultGlobal()); err != nil {
			return err
		}
	} else {
		if err := config.Save(path, config.Default()); err != nil {
			return err
		}
	}

	output.Success("Created %s", label)
	return nil
}
