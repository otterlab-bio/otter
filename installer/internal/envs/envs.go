// Package envs creates conda environments through enva or a fallback package
// manager and verifies pinned tool versions.
package envs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Pinned versions owned by the otter-core environment.
const (
	BismarkVersion = "3.1.0"
	Bowtie2Version = "2.5.5"
)

// Manager creates and verifies conda environments.
type Manager struct {
	DryRun bool
	// EnvaPath is the resolved enva binary, if available.
	EnvaPath string
	// PackageManager is the fallback conda/mamba/micromamba binary.
	PackageManager string
	// EnvFiles are the YAML files to install.
	EnvFiles []string
	// EnvsDir is the directory containing the YAML files.
	EnvsDir string
	// Choice is the environment selection: all, core, snakemake, extra.
	Choice string
}

// ResolveEnva finds an enva binary on PATH or in the install directory.
func ResolveEnva(installDir string) string {
	if path, err := exec.LookPath("enva"); err == nil {
		return path
	}
	candidate := filepath.Join(installDir, "enva")
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	return ""
}

// ResolvePackageManager finds conda, mamba, or micromamba.
func ResolvePackageManager() string {
	for _, name := range []string{"mamba", "micromamba", "conda"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

// ActiveEnvFiles returns the YAML files for the selected choice.
func (manager *Manager) ActiveEnvFiles() []string {
	core := "otter-core.yaml"
	optional := []string{"otter-snakemake.yaml", "otter-extra.yaml"}
	switch manager.Choice {
	case "all":
		return append([]string{core}, optional...)
	case "snakemake":
		return []string{core, optional[0]}
	case "extra":
		return []string{core, optional[1]}
	default:
		return []string{core}
	}
}

// Create installs the selected environments.
func (manager *Manager) Create(ctx context.Context) error {
	files := manager.ActiveEnvFiles()
	for _, yamlFile := range files {
		yamlPath := filepath.Join(manager.EnvsDir, yamlFile)
		// A dry run describes the plan, so it must not require the inputs the
		// real run would have fetched by this point.
		if !manager.DryRun {
			if _, err := os.Stat(yamlPath); err != nil {
				return fmt.Errorf("environment YAML %s not found: %w", yamlPath, err)
			}
		}
		envName := strings.TrimSuffix(yamlFile, ".yaml")
		if manager.EnvaPath != "" {
			if err := manager.createViaEnva(ctx, yamlPath, envName); err != nil {
				return err
			}
			continue
		}
		if manager.PackageManager == "" {
			if manager.DryRun {
				fmt.Printf("  [DRY-RUN] no enva or conda package manager available to create %s\n", envName)
				continue
			}
			return fmt.Errorf("no enva or conda package manager available to create %s", envName)
		}
		if err := manager.createViaPackageManager(ctx, yamlPath, envName); err != nil {
			return err
		}
	}
	return nil
}

func (manager *Manager) createViaEnva(ctx context.Context, yamlPath, envName string) error {
	args := []string{"create", "--yaml", yamlPath, "--name", envName, "--force"}
	if manager.DryRun {
		fmt.Printf("  [DRY-RUN] %s %s\n", manager.EnvaPath, strings.Join(args, " "))
		return nil
	}
	command := exec.CommandContext(ctx, manager.EnvaPath, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("create environment %s via enva: %w", envName, err)
	}
	return nil
}

func (manager *Manager) createViaPackageManager(ctx context.Context, yamlPath, envName string) error {
	if manager.DryRun {
		fmt.Printf("  [DRY-RUN] %s env create -f %s -y\n", manager.PackageManager, yamlPath)
		return nil
	}
	command := exec.CommandContext(ctx, manager.PackageManager, "env", "create", "-f", yamlPath, "-y")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("create environment %s via %s: %w", envName, manager.PackageManager, err)
	}
	return nil
}

// VerifyPinnedTools checks that otter-core provides the pinned Bismark and
// Bowtie2 versions.
func (manager *Manager) VerifyPinnedTools(ctx context.Context) error {
	if manager.DryRun {
		fmt.Printf("  [DRY-RUN] would verify bismark=%s and bowtie2=%s inside otter-core\n", BismarkVersion, Bowtie2Version)
		return nil
	}
	runner := manager.environmentRunner()
	bismarkOutput, err := runCapture(ctx, runner, "bismark", "--version")
	if err != nil {
		return fmt.Errorf("run bismark --version: %w", err)
	}
	if !strings.Contains(bismarkOutput, BismarkVersion) {
		return fmt.Errorf("otter-core does not provide Bismark %s", BismarkVersion)
	}
	bowtie2Output, err := runCapture(ctx, runner, "bowtie2", "--version")
	if err != nil {
		return fmt.Errorf("run bowtie2 --version: %w", err)
	}
	if !strings.Contains(bowtie2Output, Bowtie2Version) {
		return fmt.Errorf("otter-core does not provide Bowtie2 %s", Bowtie2Version)
	}
	return nil
}

func (manager *Manager) environmentRunner() []string {
	if manager.EnvaPath != "" {
		return []string{manager.EnvaPath, "run", "otter-core", "--"}
	}
	return []string{manager.PackageManager, "run", "-n", "otter-core", "--"}
}

func runCapture(ctx context.Context, runner []string, args ...string) (string, error) {
	commandArgs := append(append([]string{}, runner...), args...)
	command := exec.CommandContext(ctx, commandArgs[0], commandArgs[1:]...)
	output, err := command.CombinedOutput()
	return string(output), err
}
