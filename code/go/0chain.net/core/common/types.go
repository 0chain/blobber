package common

import (
	"fmt"
)

// WhoPays for file downloading.
type WhoPays int

// possible variants
const (
	WhoPaysOwner    WhoPays = iota // 0, file owner pays
	WhoPays3rdParty                // 1, 3rd party user pays
)

// String implements fmt.Stringer interface.
func (wp WhoPays) String() string {
	switch wp {
	case WhoPays3rdParty:
		return "3rd_party"
	case WhoPaysOwner:
		return "owner"
	}
	return fmt.Sprintf("WhoPays(%d)", int(wp))
}

// Validate the WhoPays value.
func (wp WhoPays) Validate() (err error) {
	switch wp {
	case WhoPays3rdParty, WhoPaysOwner:
		return // ok
	}
	return fmt.Errorf("unknown WhoPays value: %d", int(wp))
}
