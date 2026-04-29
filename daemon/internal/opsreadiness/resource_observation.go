package opsreadiness

import "fmt"

func ValidateResourceObservations(observations []ResourceObservation) error {
	coverage := map[string]bool{}
	for _, observation := range observations {
		if observation.Available {
			coverage[observation.Category] = true
		}
		if observation.MonotonicGrowth {
			return fmt.Errorf("resource category %s grew monotonically", observation.Category)
		}
		if observation.Category == "active_work_or_queue_backlog" && observation.QueueBacklogAge > MaxQueueBacklogAge {
			return fmt.Errorf("queue backlog persisted for %s", observation.QueueBacklogAge)
		}
	}
	return requireCoverage("resource observations", coverage, RequiredResourceCategories)
}
