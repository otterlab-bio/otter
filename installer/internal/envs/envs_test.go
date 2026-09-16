package envs

import (
	"reflect"
	"testing"
)

func TestActiveEnvFiles(t *testing.T) {
	testCases := []struct {
		choice   string
		expected []string
	}{
		{
			choice:   "core",
			expected: []string{"otter-core.yaml"},
		},
		{
			choice:   "",
			expected: []string{"otter-core.yaml"},
		},
		{
			choice:   "snakemake",
			expected: []string{"otter-core.yaml", "otter-snakemake.yaml"},
		},
		{
			choice:   "extra",
			expected: []string{"otter-core.yaml", "otter-extra.yaml"},
		},
		{
			choice:   "all",
			expected: []string{"otter-core.yaml", "otter-snakemake.yaml", "otter-extra.yaml"},
		},
	}

	for _, tc := range testCases {
		m := &Manager{Choice: tc.choice}
		actual := m.ActiveEnvFiles()
		if !reflect.DeepEqual(actual, tc.expected) {
			t.Errorf("ActiveEnvFiles(%q) = %v; want %v", tc.choice, actual, tc.expected)
		}
	}
}
