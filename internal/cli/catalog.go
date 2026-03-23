package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"dangernoodle.io/terra-tools/internal/catalog/catalog"
	"dangernoodle.io/terra-tools/internal/catalog/generator"
	"dangernoodle.io/terra-tools/internal/catalog/hclparse"
	"dangernoodle.io/terra-tools/internal/output"
)

const defaultTemplateFile = ".terra-generate.hcl"

func resolveTemplateFile(templateFile string) (string, error) {
	if templateFile != "" {
		if _, err := os.Stat(templateFile); err != nil {
			return "", fmt.Errorf("template file %q not found", templateFile)
		}
		return templateFile, nil
	}
	if _, err := os.Stat(defaultTemplateFile); err == nil {
		return defaultTemplateFile, nil
	}
	return "", fmt.Errorf("no template file found (expected %s or use --template-file)", defaultTemplateFile)
}

var catalogCmd = &cobra.Command{
	Use:   "catalog",
	Short: "Catalog operations for terragrunt stacks",
}

var catalogGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate terragrunt configurations from a template definition",
	RunE: func(cmd *cobra.Command, args []string) error {
		templateFile, _ := cmd.Flags().GetString("template-file")
		outputDir, _ := cmd.Flags().GetString("output-dir")
		scaffold, _ := cmd.Flags().GetBool("scaffold")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		resolvedFile, err := resolveTemplateFile(templateFile)
		if err != nil {
			return err
		}

		def, parseWarnings, err := hclparse.ParseTemplateFile(resolvedFile)
		if err != nil {
			return fmt.Errorf("parsing template file: %w", err)
		}
		for _, w := range parseWarnings {
			output.Warn("WARNING: %s", w)
		}

		catalogPath, cleanup, err := catalog.Fetch(def.CatalogSource)
		if err != nil {
			return fmt.Errorf("fetching catalog from %q: %w", def.CatalogSource, err)
		}
		defer cleanup()

		layout, err := catalog.Walk(catalogPath)
		if err != nil {
			return fmt.Errorf("walking catalog: %w", err)
		}

		errs, err := generator.Generate(&generator.Config{
			TemplateDef: def,
			Catalog:     layout,
			OutputDir:   outputDir,
			Scaffold:    scaffold,
			DryRun:      dryRun,
		})
		if err != nil {
			return fmt.Errorf("generating output: %w", err)
		}

		if len(errs) > 0 {
			for _, e := range errs {
				output.Error("ERROR: %s", e.Error())
			}
			return fmt.Errorf("%d validation error(s)", len(errs))
		}

		if !dryRun {
			output.Success("Generated output in %s", outputDir)
		}
		return nil
	},
}

var catalogScaffoldCmd = &cobra.Command{
	Use:   "scaffold",
	Short: "Generate catalog from existing terragrunt directory (not yet implemented)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("not yet implemented")
	},
}

func init() {
	catalogGenerateCmd.Flags().StringP("template-file", "t", "", "path to the template definition file (default: .terra-generate.hcl)")
	catalogGenerateCmd.Flags().StringP("output-dir", "o", ".", "where to write generated output")
	catalogGenerateCmd.Flags().Bool("scaffold", false, "create excluded directories for unconfigured catalog services")
	catalogGenerateCmd.Flags().Bool("dry-run", false, "print files that would be written without writing them")

	catalogCmd.AddCommand(catalogGenerateCmd)
	catalogCmd.AddCommand(catalogScaffoldCmd)
}
