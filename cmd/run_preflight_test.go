package cmd

import (
	"testing"

	"github.com/otterlab-bio/otter/internal/config"
)

func TestShouldPreflightRNAsplicing(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.OtterConfig
		want bool
	}{
		{
			name: "rnaseq with enough groups",
			cfg: config.OtterConfig{
				Workflow: config.WorkflowConfig{Mode: "RNASEQ"},
				Metadata: config.MetadataConfig{GroupLevels: 2},
			},
			want: true,
		},
		{
			name: "rnaseq with insufficient groups",
			cfg: config.OtterConfig{
				Workflow: config.WorkflowConfig{Mode: "RNASEQ"},
				Metadata: config.MetadataConfig{GroupLevels: 1},
			},
			want: false,
		},
		{
			name: "rrbs does not need rna preflight",
			cfg: config.OtterConfig{
				Workflow: config.WorkflowConfig{Mode: "RRBS"},
				Metadata: config.MetadataConfig{GroupLevels: 3},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := shouldPreflightRNAsplicing(&tc.cfg)
			if got != tc.want {
				t.Fatalf("shouldPreflightRNAsplicing() = %v, want %v", got, tc.want)
			}
		})
	}
}
