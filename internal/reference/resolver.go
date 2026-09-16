package reference

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

type Resolver struct {
	RegistryRoot string
	Lock         configv1.ReferencesLock
}

func LoadLock(path string) (configv1.ReferencesLock, error) {
	return configv1.LoadReferencesLock(path)
}

func (resolver Resolver) Resolve(role configv1.ReferenceRole, selection configv1.ReferenceSelection, scenario configv1.Scenario) (configv1.ResolvedReference, error) {
	lockedReference, exists := resolver.Lock.References[string(role)]
	if !exists {
		return configv1.ResolvedReference{}, fmt.Errorf("references lock has no %q entry", role)
	}
	id, release, err := configv1.ParseReferenceSelection(selection)
	if err != nil {
		return configv1.ResolvedReference{}, err
	}
	if lockedReference.ID != id || lockedReference.Release != release {
		return configv1.ResolvedReference{}, fmt.Errorf(
			"reference %q selection %s does not match lock %s@%s",
			role,
			selection,
			lockedReference.ID,
			lockedReference.Release,
		)
	}
	return resolver.resolve(role, selection, scenario, lockedReference.ManifestDigest)
}

func (resolver Resolver) ResolveOverride(role configv1.ReferenceRole, selection configv1.ReferenceSelection, scenario configv1.Scenario) (configv1.ResolvedReference, error) {
	id, release, err := configv1.ParseReferenceSelection(selection)
	if err != nil {
		return configv1.ResolvedReference{}, err
	}
	releaseRoot, _, err := resolver.locateRelease(id, release)
	if err != nil {
		return configv1.ResolvedReference{}, err
	}
	manifestDigest, err := digestFile(filepath.Join(releaseRoot, "manifest.json"))
	if err != nil {
		return configv1.ResolvedReference{}, fmt.Errorf("resolve override manifest: %w", err)
	}
	return resolver.resolve(role, selection, scenario, manifestDigest)
}

func (resolver Resolver) resolve(role configv1.ReferenceRole, selection configv1.ReferenceSelection, scenario configv1.Scenario, manifestDigest string) (configv1.ResolvedReference, error) {
	if !filepath.IsAbs(resolver.RegistryRoot) {
		return configv1.ResolvedReference{}, fmt.Errorf("reference registry root must be absolute")
	}
	id, release, err := configv1.ParseReferenceSelection(selection)
	if err != nil {
		return configv1.ResolvedReference{}, err
	}
	releaseRoot, definition, err := resolver.locateRelease(id, release)
	if err != nil {
		return configv1.ResolvedReference{}, err
	}
	if definition.Reference.Release != release || !referenceIdentityMatches(definition.Reference, id) {
		return configv1.ResolvedReference{}, fmt.Errorf("reference definition identity does not match %s", selection)
	}
	if !supportsScenario(definition.Compatibility.Scenarios, scenario) {
		return configv1.ResolvedReference{}, fmt.Errorf("reference %s does not support scenario %s", selection, scenario)
	}
	if err := VerifyReleaseIdentity(releaseRoot, manifestDigest); err != nil {
		return configv1.ResolvedReference{}, fmt.Errorf("verify reference %s identity: %w", selection, err)
	}

	resolved := configv1.ResolvedReference{
		Role:           role,
		ID:             id,
		Release:        release,
		Organism:       definition.Reference.Organism,
		RegistryRoot:   releaseRoot,
		ManifestDigest: manifestDigest,
		Fasta: configv1.ResolvedAsset{
			Type:   "fasta",
			Path:   filepath.Join(releaseRoot, definition.Assets.Fasta.Path),
			SHA256: definition.Assets.Fasta.SHA256,
		},
	}
	for _, annotation := range definition.Assets.Annotations {
		resolved.Annotations = append(resolved.Annotations, configv1.ResolvedAsset{
			Type:   annotation.Type,
			Path:   filepath.Join(releaseRoot, annotation.Path),
			SHA256: annotation.SHA256,
		})
	}
	for _, index := range definition.Assets.Indexes {
		resolved.Indexes = append(resolved.Indexes, configv1.ResolvedAsset{
			Type:   index.Type,
			Path:   filepath.Join(releaseRoot, index.Path),
			SHA256: index.ManifestSHA256,
		})
	}
	return resolved, nil
}

func (resolver Resolver) locateRelease(requestedID, release string) (string, configv1.ReferenceDefinition, error) {
	if !filepath.IsAbs(resolver.RegistryRoot) {
		return "", configv1.ReferenceDefinition{}, fmt.Errorf("reference registry root must be absolute")
	}

	directReleaseRoot := filepath.Join(resolver.RegistryRoot, "genomes", requestedID, release)
	directDefinitionPath := filepath.Join(directReleaseRoot, "reference.yaml")
	if _, err := os.Stat(directDefinitionPath); err == nil {
		definition, loadErr := configv1.LoadReferenceDefinition(directDefinitionPath)
		if loadErr != nil {
			return "", configv1.ReferenceDefinition{}, loadErr
		}
		return directReleaseRoot, definition, nil
	} else if !os.IsNotExist(err) {
		return "", configv1.ReferenceDefinition{}, fmt.Errorf("inspect reference definition %s: %w", directDefinitionPath, err)
	}

	genomesRoot := filepath.Join(resolver.RegistryRoot, "genomes")
	genomeEntries, err := os.ReadDir(genomesRoot)
	if err != nil {
		return "", configv1.ReferenceDefinition{}, fmt.Errorf("list reference registry %s: %w", genomesRoot, err)
	}

	var aliasReleaseRoot string
	var aliasDefinition configv1.ReferenceDefinition
	for _, genomeEntry := range genomeEntries {
		if !genomeEntry.IsDir() || genomeEntry.Name() == requestedID {
			continue
		}
		candidateReleaseRoot := filepath.Join(genomesRoot, genomeEntry.Name(), release)
		candidateDefinitionPath := filepath.Join(candidateReleaseRoot, "reference.yaml")
		if _, statErr := os.Stat(candidateDefinitionPath); os.IsNotExist(statErr) {
			continue
		} else if statErr != nil {
			return "", configv1.ReferenceDefinition{}, fmt.Errorf("inspect reference definition %s: %w", candidateDefinitionPath, statErr)
		}
		candidateDefinition, loadErr := configv1.LoadReferenceDefinition(candidateDefinitionPath)
		if loadErr != nil {
			return "", configv1.ReferenceDefinition{}, loadErr
		}
		if candidateDefinition.Reference.Release != release || !referenceIdentityMatches(candidateDefinition.Reference, requestedID) {
			continue
		}
		if aliasReleaseRoot != "" {
			return "", configv1.ReferenceDefinition{}, fmt.Errorf("reference alias %q@%s is ambiguous between %s and %s", requestedID, release, aliasReleaseRoot, candidateReleaseRoot)
		}
		aliasReleaseRoot = candidateReleaseRoot
		aliasDefinition = candidateDefinition
	}
	if aliasReleaseRoot == "" {
		return "", configv1.ReferenceDefinition{}, fmt.Errorf("reference %q@%s is not present in registry %s", requestedID, release, resolver.RegistryRoot)
	}
	return aliasReleaseRoot, aliasDefinition, nil
}

func referenceIdentityMatches(identity configv1.ReferenceIdentity, requestedID string) bool {
	if identity.ID == requestedID {
		return true
	}
	for _, alias := range identity.Aliases {
		if alias == requestedID {
			return true
		}
	}
	return false
}

func digestFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

func supportsScenario(scenarios []configv1.Scenario, expected configv1.Scenario) bool {
	for _, scenario := range scenarios {
		if scenario == expected {
			return true
		}
	}
	return false
}

func formatVerificationIssues(issues []VerificationIssue) string {
	if len(issues) == 0 {
		return "unknown verification failure"
	}
	firstIssue := issues[0]
	return fmt.Sprintf("%s %s: expected %s, got %s", firstIssue.Asset, firstIssue.Path, firstIssue.Expected, firstIssue.Actual)
}
