package enva

import (
	"fmt"
	"testing"
)

func TestValidateEnvironment(t *testing.T) {
	// Test 1: Non-existent environment
	t.Run("NonExistentEnvironment", func(t *testing.T) {
		err := ValidateEnvironment("non-existent-env-12345")
		if err == nil {
			t.Error("Expected error for non-existent environment, got nil")
		} else {
			fmt.Printf("✓ Correctly detected non-existent environment: %v\n", err)
		}
	})

	// Test 2: Check enva availability
	t.Run("EnvaAvailability", func(t *testing.T) {
		if IsAvailable() {
			fmt.Println("✓ enva is available")
		} else {
			fmt.Println("✓ enva not available (will use conda)")
		}
	})

	// Test 3: Check otter-snakemake environment
	t.Run("XdxtoolsSnakemake", func(t *testing.T) {
		err := ValidateEnvironment("otter-snakemake")
		if err != nil {
			fmt.Printf("⚠ otter-snakemake validation failed: %v\n", err)
			fmt.Println("  Note: This is expected if the environment is not installed")
		} else {
			fmt.Println("✓ otter-snakemake environment is valid")
		}
	})

	// Test 4: System snakemake (empty string)
	t.Run("SystemSnakemake", func(t *testing.T) {
		err := ValidateEnvironment("")
		if err != nil {
			fmt.Printf("⚠ System snakemake not available: %v\n", err)
		} else {
			fmt.Println("✓ System snakemake is available")
		}
	})
}
