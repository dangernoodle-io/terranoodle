package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"dangernoodle.io/terratools/internal/catalog/catalog"
	"dangernoodle.io/terratools/internal/catalog/generator"
	"dangernoodle.io/terratools/internal/catalog/hclparse"
	"dangernoodle.io/terratools/internal/output"
)

var catalogCmd = &cobra.Command{
	Use:   "catalog",
	Short: "Catalog management commands",
}

var (
	templateFlag string
	catalogFlag  string
	outputFlag   string
	dryRunFlag   bool
	scaffoldFlag bool
)

var catalogGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a terragrunt stack from a catalog template",
	RunE:  runCatalogGenerate,
}

func init() {
	catalogGenerateCmd.Flags().StringVarP(&templateFlag, "template", "t", "", "Path to template file (required)")
	catalogGenerateCmd.Flags().StringVarP(&catalogFlag, "catalog", "c", "", "Path or URL to catalog (required)")
	catalogGenerateCmd.Flags().StringVarP(&outputFlag, "output", "o", "", "Output directory (required)")
	catalogGenerateCmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "Print changes without writing files")
	catalogGenerateCmd.Flags().BoolVar(&scaffoldFlag, "scaffold", false, "Write scaffold stubs for missing services")

	_ = catalogGenerateCmd.MarkFlagRequired("template")
	_ = catalogGenerateCmd.MarkFlagRequired("catalog")
	_ = catalogGenerateCmd.MarkFlagRequired("output")

	catalogCmd.AddCommand(catalogGenerateCmd)
}

func runCatalogGenerate(cmd *cobra.Command, args []string) error {
	tmplDef, warnings, err := hclparse.ParseTemplateFile(templateFlag)
	if err != nil {
		return err
	}

	for _, w := range warnings {
		output.Warn("%s", w)
	}

	catalogPath, cleanup, err := catalog.Fetch(catalogFlag)
	if err != nil {
		return err
	}
	defer cleanup()

	layout, err := catalog.Walk(catalogPath)
	if err != nil {
		return err
	}

	cfg := generator.Config{
		TemplateDef: tmplDef,
		Catalog:     layout,
		OutputDir:   outputFlag,
		Scaffold:    scaffoldFlag,
		DryRun:      dryRunFlag,
	}

	errs, err := generator.Generate(&cfg)
	if err != nil {
		return err
	}

	if len(errs) > 0 {
		for _, e := range errs {
			output.Error("%s", e.Error())
		}
		return fmt.Errorf("generation failed with %d error(s)", len(errs))
	}

	output.Success("Generation complete")
	return nil
}
