package esi

const (
	// Capability is the capability token for ESI processors.
	Capability = "ESI/1.0"

	// InlineCapability is the capability token for ESI processors that support the <esi:inline> element.
	InlineCapability = "ESI-Inline/1.0"
)

// ErrorBehaviour defines the valid values for the "onerror" attribute of the <esi:include> tag.
type ErrorBehaviour string

const (
	// ErrorBehaviourContinue means that the processing should continue on errors.
	ErrorBehaviourContinue ErrorBehaviour = "continue"

	// ErrorBehaviourDefault means no error behaviour was selected.
	ErrorBehaviourDefault ErrorBehaviour = ""
)

// String returns the name of the behaviour.
func (e ErrorBehaviour) String() string {
	switch e {
	case ErrorBehaviourContinue:
		return "ErrorBehaviourContinue"
	case ErrorBehaviourDefault:
		return "ErrorBehaviourDefault"
	default:
		panic("unknown error behaviour")
	}
}
