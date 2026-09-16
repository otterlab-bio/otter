// Command stub-registry writes simulated Otter reference registry releases
// without downloading a genome or running a genome index builder.
//
// It exists so end-to-end tests can exercise "otter create" and
// "otter config resolve" against the real reference contract while staying
// offline. Every release it writes is verified with the production reference
// verifier before the command reports success, so a drift in the reference
// contract fails here rather than producing a quietly invalid fixture.
//
// Usage:
//
//	stub-registry --registry-root <dir> --id <reference-id> --release <release> \
//	  --organism 'Homo sapiens' --assembly hg19 [--indexes bismark,bowtie2,star] \
//	  [--alias <id>] [--scenario rrbs] [--print-digest]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
	"github.com/otterlab-bio/otter/internal/e2esupport"
)

func main() {
	var (
		registryRoot string
		referenceID  string
		release      string
		organism     string
		assembly     string
		indexList    string
		aliasList    string
		scenarioList string
		printAsJSON  bool
	)

	flag.StringVar(&registryRoot, "registry-root", "", "Reference registry root (required); the release lands at <root>/genomes/<id>/<release>")
	flag.StringVar(&referenceID, "id", "", "Logical reference id, for example hg19 (required)")
	flag.StringVar(&release, "release", "", "Immutable release label, for example hg19-legacy (required)")
	flag.StringVar(&organism, "organism", "", "Scientific organism name, for example 'Homo sapiens' (required)")
	flag.StringVar(&assembly, "assembly", "", "Assembly label, for example hg19 (required)")
	flag.StringVar(&indexList, "indexes", "", "Comma-separated index types to materialize: bismark,bowtie2,star (default: all)")
	flag.StringVar(&aliasList, "alias", "", "Comma-separated alternate reference ids that resolve to this release")
	flag.StringVar(&scenarioList, "scenario", "", "Comma-separated compatibility scenarios; default derives from indexes")
	flag.BoolVar(&printAsJSON, "json", false, "Print the result as JSON instead of text")
	flag.Parse()

	if registryRoot == "" || referenceID == "" || release == "" || organism == "" || assembly == "" {
		fmt.Fprintln(os.Stderr, "error: --registry-root, --id, --release, --organism, and --assembly are all required")
		flag.Usage()
		os.Exit(2)
	}

	// Default to every index type so a single stub release can serve all five
	// scenarios; narrow it explicitly when a test needs a scenario-specific gap.
	indexes := splitList(indexList)
	if len(indexes) == 0 {
		indexes = []string{"bismark", "bowtie2", "star"}
	}
	scenarios := make([]configv1.Scenario, 0)
	for _, scenario := range splitList(scenarioList) {
		scenarios = append(scenarios, configv1.Scenario(scenario))
	}

	result, err := e2esupport.WriteStubRelease(e2esupport.StubReleaseRequest{
		RegistryRoot: registryRoot,
		ReferenceID:  referenceID,
		Release:      release,
		Organism:     organism,
		Assembly:     assembly,
		Aliases:      splitList(aliasList),
		Indexes:      indexes,
		Scenarios:    scenarios,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if printAsJSON {
		payload := struct {
			ReleaseRoot    string   `json:"release_root"`
			DefinitionPath string   `json:"definition_path"`
			ManifestPath   string   `json:"manifest_path"`
			ManifestDigest string   `json:"manifest_digest"`
			Scenarios      []string `json:"scenarios"`
		}{
			ReleaseRoot:    result.ReleaseRoot,
			DefinitionPath: result.DefinitionPath,
			ManifestPath:   result.ManifestPath,
			ManifestDigest: result.ManifestDigest,
		}
		for _, scenario := range result.Scenarios {
			payload.Scenarios = append(payload.Scenarios, string(scenario))
		}
		encoded, marshalErr := json.MarshalIndent(payload, "", "  ")
		if marshalErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", marshalErr)
			os.Exit(1)
		}
		fmt.Println(string(encoded))
		return
	}

	fmt.Printf("Stub reference release written: %s\n", result.ReleaseRoot)
	fmt.Printf("  definition:      %s\n", result.DefinitionPath)
	fmt.Printf("  manifest:        %s\n", result.ManifestPath)
	fmt.Printf("  manifest digest: %s\n", result.ManifestDigest)
	fmt.Printf("  scenarios:       %s\n", joinScenarios(result.Scenarios))
	fmt.Printf("  verification:    production reference identity verification passed\n")
}

// configv1Scenario is a local alias kept out of the public surface; the flag
// parses into configv1.Scenario directly.
func splitList(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func joinScenarios(scenarios []configv1.Scenario) string {
	if len(scenarios) == 0 {
		return "<none>"
	}
	items := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		items = append(items, string(scenario))
	}
	return strings.Join(items, ",")
}
