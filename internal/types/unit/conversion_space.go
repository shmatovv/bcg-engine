package unit

// ConversionSpaceID identifies a family of units between which conversion
// is fixed, exact and context-free.
//
// Units may share the same Dimension while belonging to different conversion
// spaces.
//
// Examples:
//
//	meter, centimeter   -> "length"
//	kilogram, gram      -> "mass"
//	ruble, kopeck       -> "rub"
//	euro, euro_cent     -> "eur"
//
// RUB and EUR may both have the Currency Dimension, but they belong to
// different conversion spaces because converting between them requires
// external context such as an exchange rate.
type ConversionSpaceID string

// IsValid reports whether id is a valid conversion-space identifier.
//
// The syntax intentionally matches UnitID:
//   - non-empty;
//   - starts with a lowercase ASCII letter;
//   - contains lowercase ASCII letters, digits and underscores.
func (id ConversionSpaceID) IsValid() bool {
	value := string(id)

	if len(value) == 0 {
		return false
	}

	if value[0] < 'a' || value[0] > 'z' {
		return false
	}

	for i := 1; i < len(value); i++ {
		c := value[i]

		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '_':
		default:
			return false
		}
	}

	return true
}

// ConversionFactor represents one conversion space raised to an integer
// exponent.
//
// Compound units compose conversion spaces algebraically.
//
// Examples:
//
//	RUB/item
//	    rub   ->  1
//	    count -> -1
//
//	kg*m/s²
//	    mass   ->  1
//	    length ->  1
//	    time   -> -2
type ConversionFactor struct {
	id       ConversionSpaceID
	exponent int
}
