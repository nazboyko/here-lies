package obit

// Notice replaces the obituary when the photo shows something alive or
// nothing we can make out.
type Notice struct {
	Subject string `json:"subject"`
	Title   string `json:"title"`
	Line    string `json:"line"`
}

// Gate returns a notice for anything that is not a thing, nil otherwise.
func Gate(l Look) *Notice {
	switch l.Subject {
	case Thing:
		return nil
	case Person:
		return &Notice{Person, "We don't write obituaries for people.", "This one looks very much alive."}
	case Animal:
		return &Notice{Animal, "Still alive.", "Give it a treat instead."}
	case LivingPlant:
		return &Notice{LivingPlant, "This plant is still with us.", "Water it."}
	default:
		return &Notice{Unclear, "We couldn't make out the deceased.", "Try a closer photo in better light."}
	}
}
