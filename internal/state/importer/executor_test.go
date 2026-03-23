package importer

import (
	"context"
	"strings"
	"testing"
)

func TestGeneratePlan_BinaryNotFound(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := GeneratePlan(context.Background(), ".", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "terraform") {
		t.Errorf("expected error to contain %q, got: %s", "terraform", err.Error())
	}
}

func TestTerragruntGeneratePlan_BinaryNotFound(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := TerragruntGeneratePlan(context.Background(), ".", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "terragrunt") {
		t.Errorf("expected error to contain %q, got: %s", "terragrunt", err.Error())
	}
}
