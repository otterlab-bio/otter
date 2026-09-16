package main

import (
	"embed"

	"github.com/otterlab-bio/otter/internal/assets"
)

// Packaged workflow, environment, and schema assets that `otter init` pins into
// a canonical project. The schema files are embedded so a project carries the
// exact contract it was created against.
//
//go:embed inst/Rscripts/* inst/snakefiles/* inst/rules/* inst/rules_legacy/* inst/envs/* inst/data/* docs/schema/*
var embeddedAssets embed.FS

func init() {
	assets.SetEmbeddedAssets(embeddedAssets)
}
