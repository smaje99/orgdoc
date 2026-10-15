package scheduler

import "github.com/smaje/orgdoc/internal/worker"

const DefaultMaxWorkers = 4

// ResourceLimits bounds total active jobs and active work within each class.
// CPU jobs default to MaxWorkers; GPU and memory-heavy work default to one each.
type ResourceLimits struct {
	MaxWorkers int
	ByClass    map[worker.ResourceClass]int
}

func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxWorkers: DefaultMaxWorkers,
		ByClass: map[worker.ResourceClass]int{
			worker.ResourceClassCPU:    DefaultMaxWorkers,
			worker.ResourceClassGPU:    1,
			worker.ResourceClassMemory: 1,
		},
	}
}

// ResourceLimitsForMaxWorkers applies the CLI override to the global and CPU limits.
// GPU/model and memory-heavy work remain bounded to one unless configured separately.
func ResourceLimitsForMaxWorkers(maxWorkers int) ResourceLimits {
	limits := DefaultResourceLimits()
	if maxWorkers > 0 {
		limits.MaxWorkers = maxWorkers
		limits.ByClass[worker.ResourceClassCPU] = maxWorkers
	}
	return limits
}

func normalizeLimits(limits ResourceLimits) ResourceLimits {
	defaults := DefaultResourceLimits()
	if limits.MaxWorkers > 0 {
		defaults.MaxWorkers = limits.MaxWorkers
	}
	for class, limit := range limits.ByClass {
		if limit > 0 {
			defaults.ByClass[class] = limit
		}
	}
	if _, configured := limits.ByClass[worker.ResourceClassCPU]; !configured && limits.MaxWorkers > 0 {
		defaults.ByClass[worker.ResourceClassCPU] = limits.MaxWorkers
	}
	return defaults
}
