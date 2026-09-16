package workflow

import (
	"testing"

	"github.com/otterlab-bio/otter/internal/config"
)

func TestManagerSetSamples(t *testing.T) {
	cfg := &config.OtterConfig{
		Workflow: config.WorkflowConfig{
			Mode: "RRBS",
		},
	}

	w := NewWorkflow(cfg, []string{})
	manager := NewManager(w)

	samples := []string{"sample1", "sample2", "sample3", "sample4", "sample5", "sample6", "sample7", "sample8"}
	manager.SetSamples(samples)

	if len(manager.samples) != 8 {
		t.Errorf("Expected 8 samples, got %d", len(manager.samples))
	}
}

func TestManagerSetStepResources(t *testing.T) {
	cfg := &config.OtterConfig{
		Workflow: config.WorkflowConfig{
			Mode: "RRBS",
		},
	}

	w := NewWorkflow(cfg, []string{})
	manager := NewManager(w)

	resources := map[int]*config.StepResource{
		1: {Cores: 4, Memory: "8G", Partition: "cpu"},
		2: {Cores: 16, Memory: "32G", Partition: "cpu112c", JobArray: true},
		3: {Cores: 8, Memory: "16G", Partition: "cpu112c", JobArray: true},
	}

	manager.SetStepResources(resources)

	if len(manager.stepResources) != 3 {
		t.Errorf("Expected 3 resources, got %d", len(manager.stepResources))
	}
}

func TestManagerGetStepResource(t *testing.T) {
	cfg := &config.OtterConfig{
		Workflow: config.WorkflowConfig{
			Mode: "RRBS",
		},
	}

	w := NewWorkflow(cfg, []string{})
	manager := NewManager(w)

	resources := map[int]*config.StepResource{
		2: {Cores: 16, Memory: "32G", Partition: "cpu112c"},
	}

	manager.SetStepResources(resources)

	// Test explicit resource
	resource2 := manager.getStepResource(2)
	if resource2 == nil || resource2.Cores != 16 {
		t.Errorf("Expected resource for step 2, got %v", resource2)
	}

	// Test inherited resource (step 2 checker should inherit from step 2)
	resource102 := manager.getStepResource(102)
	if resource102 == nil || resource102.Cores != 16 {
		t.Errorf("Expected inherited resource for step 2 checker, got %v", resource102)
	}

	// Test default resource (step 1 not configured)
	resource1 := manager.getStepResource(1)
	if resource1 == nil {
		t.Error("Expected default resource for step 1, got nil")
	}
	if resource1.Cores != 20 {
		t.Errorf("Expected default cores 20, got %d", resource1.Cores)
	}
}

func TestManagerIntelligentStrategy(t *testing.T) {
	cfg := &config.OtterConfig{
		Workflow: config.WorkflowConfig{
			Mode: "RRBS",
		},
	}

	w := NewWorkflow(cfg, []string{})
	manager := NewManager(w)

	// Parallelization is now controlled by parallelJobs > 1 (not sample count)
	// shouldUseSingleSampleMode returns true for step 2 and 3 (uses per-sample mode)
	// and false for step 1 (uses all-samples mode)
	if manager.shouldUseSingleSampleMode(1) {
		t.Error("Expected all-samples mode for step 1 (shouldUseSingleSampleMode=false)")
	}

	if !manager.shouldUseSingleSampleMode(2) {
		t.Error("Expected single-sample mode for step 2 (shouldUseSingleSampleMode=true)")
	}

	if !manager.shouldUseSingleSampleMode(3) {
		t.Error("Expected single-sample mode for step 3 (shouldUseSingleSampleMode=true)")
	}

	// Large sample counts should still work
	samples := []string{"sample1", "sample2", "sample3", "sample4", "sample5", "sample6", "sample7", "sample8", "sample9", "sample10"}
	manager.SetSamples(samples)

	if !manager.shouldUseSingleSampleMode(2) {
		t.Error("Expected single-sample mode for step 2 regardless of sample count")
	}
}

func TestManagerShouldUseSingleSampleMode(t *testing.T) {
	cfg := &config.OtterConfig{
		Workflow: config.WorkflowConfig{
			Mode: "RRBS",
		},
	}

	w := NewWorkflow(cfg, []string{})
	manager := NewManager(w)

	// Step 2 and 3 should use single-sample mode
	if !manager.shouldUseSingleSampleMode(2) {
		t.Error("Expected step 2 to use single-sample mode")
	}
	if !manager.shouldUseSingleSampleMode(3) {
		t.Error("Expected step 3 to use single-sample mode")
	}

	// Step 1 should NOT use single-sample mode
	if manager.shouldUseSingleSampleMode(1) {
		t.Error("Expected step 1 to use all-samples mode")
	}

	// Checker steps should NOT use single-sample mode
	if manager.shouldUseSingleSampleMode(102) {
		t.Error("Expected step 2 checker to use all-samples mode")
	}
	if manager.shouldUseSingleSampleMode(103) {
		t.Error("Expected step 3 checker to use all-samples mode")
	}
}

func TestManagerCheckerInheritance(t *testing.T) {
	cfg := &config.OtterConfig{
		Workflow: config.WorkflowConfig{
			Mode: "RRBS",
		},
	}

	w := NewWorkflow(cfg, []string{})
	manager := NewManager(w)

	resources := map[int]*config.StepResource{
		2: {Cores: 16, Memory: "32G", Partition: "cpu112c"},
		3: {Cores: 8, Memory: "16G", Partition: "cpu112c"},
	}

	manager.SetStepResources(resources)

	// Step 2 Checker should inherit from Step 2
	resource102 := manager.getStepResource(102)
	if resource102 == nil {
		t.Error("Expected step 2 checker to inherit resource")
	}
	if resource102.Cores != 16 {
		t.Errorf("Expected step 2 checker to inherit cores 16, got %d", resource102.Cores)
	}

	// Step 3 Checker should inherit from Step 3
	resource103 := manager.getStepResource(103)
	if resource103 == nil {
		t.Error("Expected step 3 checker to inherit resource")
	}
	if resource103.Cores != 8 {
		t.Errorf("Expected step 3 checker to inherit cores 8, got %d", resource103.Cores)
	}
}
