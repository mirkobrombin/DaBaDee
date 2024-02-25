package dabadee

import "github.com/mirkobrombin/dabadee/pkg/processor"

type DaBaDee struct {
	Processor processor.Processor
}

// NewDaBaDee creates a new DaBaDee orchestrator with the given processor
func NewDaBaDee(p processor.Processor) *DaBaDee {
	return &DaBaDee{Processor: p}
}

// Run starts the given processor
func (d *DaBaDee) Run() error {
	return d.Processor.Process()
}
