package dabadee

import "github.com/mirkobrombin/dabadee/pkg/processor"

type DaBaDee struct {
	Processor processor.Processor
	Verbose   bool
}

// NewDaBaDee creates a new DaBaDee orchestrator with the given processor
func NewDaBaDee(p processor.Processor, verbose bool) *DaBaDee {
	return &DaBaDee{
		Processor: p,
		Verbose:   verbose,
	}
}

// Run starts the given processor
func (d *DaBaDee) Run() error {
	return d.Processor.Process(d.Verbose)
}
