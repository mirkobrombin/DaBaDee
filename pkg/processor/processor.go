package processor

// Processor defines the core processing logic, implement this to add new
// functionalities that can be orchestrated by DaBaDee
type Processor interface {
	Process(verbose bool) error
}
