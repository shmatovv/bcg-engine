package value

// Type identifies the kind of value represented by the engine.
type Type int

const (
	// Invalid represents an invalid value type.
	Invalid Type = iota

	// Bool represents a boolean value.
	Bool

	// Integer represents an integer value.
	Integer

	// Decimal represents an arbitrary-precision decimal value.
	Decimal

	// String represents a textual value.
	String

	// Date represents a calendar date without a time of day.
	Date

	// DateTime represents a date and time.
	DateTime

	// Duration represents an elapsed or requested amount of time.
	Duration

	// Enum represents one option of a schema-owned EnumDefinition. The
	// definition identity is carried by Raw/Typed rather than this broad kind.
	Enum

	last
)

// IsValid reports whether t is a valid Type.
func (t Type) IsValid() bool {
	return t > Invalid && t < last
}

// String returns the name of t.
//
// String returns "Invalid" for values that do not represent a valid Type.
func (t Type) String() string {
	switch t {
	case Bool:
		return "Bool"
	case Integer:
		return "Integer"
	case Decimal:
		return "Decimal"
	case String:
		return "String"
	case Date:
		return "Date"
	case DateTime:
		return "DateTime"
	case Duration:
		return "Duration"
	case Enum:
		return "Enum"
	default:
		return "Invalid"
	}
}
