package model

import (
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/graph/dependency"
)

// ConstraintDependencies is the immutable concrete property dependency set
// of one Constraint. Applicability and Assertion remain distinct because the
// former can stop the latter from executing at runtime.
//
// These are structural dependencies. They intentionally include both lazy
// branches of each expression; actual execution is available from the normal
// evaluator provenance retained in constraint/check.Result.
type ConstraintDependencies struct {
	applicability []address.PropertyAddress
	assertion     []address.PropertyAddress
}

func newConstraintDependencies(applicability, assertion []address.PropertyAddress) ConstraintDependencies {
	result := ConstraintDependencies{
		applicability: append([]address.PropertyAddress(nil), applicability...),
		assertion:     append([]address.PropertyAddress(nil), assertion...),
	}
	if !result.IsValid() {
		panic("model: invalid constraint dependency index entry")
	}
	return result
}

// Applicability returns an owned deterministic list of properties structurally
// read by a Constraint's applicability expression.
func (d ConstraintDependencies) Applicability() []address.PropertyAddress {
	return append([]address.PropertyAddress(nil), d.applicability...)
}

// Assertion returns an owned deterministic list of properties structurally
// read by a Constraint's assertion expression.
func (d ConstraintDependencies) Assertion() []address.PropertyAddress {
	return append([]address.PropertyAddress(nil), d.assertion...)
}

// All returns the deterministic unique union of Applicability and Assertion
// dependencies. It is useful for property-to-constraint structural analysis.
func (d ConstraintDependencies) All() []address.PropertyAddress {
	result := append([]address.PropertyAddress(nil), d.applicability...)
	result = append(result, d.assertion...)
	sortDependencyPropertyAddresses(result)
	unique := result[:0]
	for _, item := range result {
		if len(unique) == 0 || unique[len(unique)-1] != item {
			unique = append(unique, item)
		}
	}
	return append([]address.PropertyAddress(nil), unique...)
}

// IsValid reports whether the dependency lists contain valid, canonical,
// duplicate-free PropertyAddress values.
func (d ConstraintDependencies) IsValid() bool {
	return validCanonicalDependencyAddresses(d.applicability) && validCanonicalDependencyAddresses(d.assertion)
}

// ConstraintDependencies resolves one committed Constraint's indexed
// structural dependencies. false means constraintID is absent from version.
func (v ModelVersion) ConstraintDependencies(constraintID identity.UUID) (ConstraintDependencies, bool) {
	dependencies, ok := v.indexes.constraintDependenciesByID[constraintID]
	return dependencies, ok
}

// ConstraintIDsDependingOn returns an owned deterministic list of Constraints
// that structurally read propertyAddress in their applicability or assertion.
func (v ModelVersion) ConstraintIDsDependingOn(propertyAddress address.PropertyAddress) []identity.UUID {
	if !propertyAddress.IsValid() {
		return nil
	}
	return append([]identity.UUID(nil), v.indexes.constraintIDsByDependency[propertyAddress]...)
}

func buildConstraintDependencyIndexes(version ModelVersion) (map[identity.UUID]ConstraintDependencies, map[address.PropertyAddress][]identity.UUID, error) {
	byConstraint := make(map[identity.UUID]ConstraintDependencies, len(version.constraints))
	byProperty := make(map[address.PropertyAddress][]identity.UUID)
	constraints := append([]constraint.Constraint(nil), version.constraints...)
	sort.Slice(constraints, func(i, j int) bool { return compareDependencyUUID(constraints[i].ID(), constraints[j].ID()) < 0 })
	for _, modelConstraint := range constraints {
		var applicability []address.PropertyAddress
		if expression, hasApplicability := modelConstraint.Applicability(); hasApplicability {
			var err error
			applicability, err = dependency.ExtractStandaloneExpression(version, expression)
			if err != nil {
				return nil, nil, err
			}
		}
		assertion, err := dependency.ExtractStandaloneExpression(version, modelConstraint.Assertion())
		if err != nil {
			return nil, nil, err
		}
		dependencies := newConstraintDependencies(applicability, assertion)
		byConstraint[modelConstraint.ID()] = dependencies
		for _, propertyAddress := range dependencies.All() {
			byProperty[propertyAddress] = append(byProperty[propertyAddress], modelConstraint.ID())
		}
	}
	for propertyAddress := range byProperty {
		ids := byProperty[propertyAddress]
		sort.Slice(ids, func(i, j int) bool { return compareDependencyUUID(ids[i], ids[j]) < 0 })
		byProperty[propertyAddress] = ids
	}
	return byConstraint, byProperty, nil
}

func validCanonicalDependencyAddresses(values []address.PropertyAddress) bool {
	for index, item := range values {
		if !item.IsValid() || (index > 0 && !lessDependencyAddress(values[index-1], item)) {
			return false
		}
	}
	return true
}

func sortDependencyPropertyAddresses(values []address.PropertyAddress) {
	sort.Slice(values, func(i, j int) bool { return lessDependencyAddress(values[i], values[j]) })
}

func lessDependencyAddress(left, right address.PropertyAddress) bool {
	if left.Object().Kind() != right.Object().Kind() {
		return left.Object().Kind() < right.Object().Kind()
	}
	if comparison := compareDependencyUUID(left.Object().ID(), right.Object().ID()); comparison != 0 {
		return comparison < 0
	}
	return compareDependencyUUID(left.DefinitionID(), right.DefinitionID()) < 0
}

func compareDependencyUUID(left, right identity.UUID) int {
	leftBytes, rightBytes := left.Bytes(), right.Bytes()
	for index := range leftBytes {
		if leftBytes[index] < rightBytes[index] {
			return -1
		}
		if leftBytes[index] > rightBytes[index] {
			return 1
		}
	}
	return 0
}
