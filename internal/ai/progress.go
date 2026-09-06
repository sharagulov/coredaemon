package ai

// Phase is a live update from the agent loop.
type Phase struct {
	Kind     string  `json:"kind"` // thinking | created | updated
	File     string  `json:"file,omitempty"`
	Title    string  `json:"title,omitempty"`
	Previous *string `json:"previous,omitempty"`
}

// ProgressFunc receives agent phase updates. Nil is safe to pass.
type ProgressFunc func(Phase)

func emitProgress(fn ProgressFunc, phase Phase) {
	if fn != nil {
		fn(phase)
	}
}
