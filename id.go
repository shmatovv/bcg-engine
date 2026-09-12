package bcg

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/internal/domain/identity"
)

// ID is the stable UUID identity used for all BCG model objects.
// Its zero value is the nil UUID and is not a valid model-object identity.
type ID struct{ value identity.UUID }

// ParseID parses a canonical UUID.
func ParseID(text string) (ID, error) {
	parsed, err := identity.Parse(text)
	if err != nil {
		return ID{}, fmt.Errorf("%w: ID: %v", ErrInvalidArgument, err)
	}
	return idFromInternal(parsed), nil
}

// MustParseID parses a canonical UUID or panics. It is intended for constants
// and tests, not untrusted input.
func MustParseID(text string) ID {
	result, err := ParseID(text)
	if err != nil {
		panic(err)
	}
	return result
}

// NewID creates a random UUID suitable for a new external identifier.
func NewID() ID { return idFromInternal(identity.New()) }

// String returns canonical UUID text.
func (id ID) String() string { return id.value.String() }

// IsZero reports whether id is the nil UUID.
func (id ID) IsZero() bool { return id.value.IsZero() }

// Equal reports semantic UUID equality.
func (id ID) Equal(other ID) bool { return id.value.Equal(other.value) }

func idFromInternal(id identity.UUID) ID { return ID{value: id} }

func (id ID) internal() (identity.UUID, error) {
	if id.IsZero() {
		return identity.UUID{}, fmt.Errorf("%w: ID must not be zero", ErrInvalidArgument)
	}
	return id.value, nil
}
