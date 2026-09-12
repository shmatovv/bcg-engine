package model

import (
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/constraint"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/types/precision"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func boolConstant(flag bool) ast.Expression {
	return ast.NewConstant(value.NewTyped(value.NewBool(flag)))
}

func intConstant(number int64) ast.Expression {
	return ast.NewConstant(value.NewTyped(value.NewInt64(number)))
}

func TestDraftValidateConstraintBooleanContractsAndIdentity(t *testing.T) {
	t.Run("non boolean assertion", func(t *testing.T) {
		draft := NewDraft()
		draft.AddConstraint(constraint.New(identity.New(), "bad", nil, false, intConstant(1), constraint.SeverityHard, "", false))
		requireValidationIssue(t, draft.Validate(), ValidationIssueInvalidConstraintAssertion)
	})
	t.Run("non boolean applicability", func(t *testing.T) {
		draft := NewDraft()
		draft.AddConstraint(constraint.New(identity.New(), "bad", intConstant(1), true, boolConstant(true), constraint.SeverityHard, "", false))
		requireValidationIssue(t, draft.Validate(), ValidationIssueInvalidConstraintApplicability)
	})
	t.Run("duplicate ID", func(t *testing.T) {
		draft := NewDraft()
		id := identity.New()
		draft.AddConstraint(constraint.New(id, "one", nil, false, boolConstant(true), constraint.SeverityHard, "", false))
		draft.AddConstraint(constraint.New(id, "two", nil, false, boolConstant(false), constraint.SeverityHard, "", false))
		requireValidationIssue(t, draft.Validate(), ValidationIssueDuplicateConstraintID)
	})
	t.Run("series bool assertion is rejected without implicit all any", func(t *testing.T) {
		fixture := newBaseValueFixture(t, schema.InputComputation(), schema.TemporalSeries, identity.UUID{}, false, value.Bool, unit.Unit{}, false)
		target := address.NewPropertyAddress(address.EntityRef(fixture.entityID), fixture.definitionID)
		fixture.draft.AddConstraint(constraint.New(identity.New(), "series", nil, false, ast.NewAbsolutePropertyReference(target), constraint.SeverityHard, "", false))
		requireValidationIssue(t, fixture.draft.Validate(), ValidationIssueInvalidConstraintAssertion)
	})
	t.Run("invalid absolute reference", func(t *testing.T) {
		draft := NewDraft()
		missing := address.NewPropertyAddress(address.EntityRef(identity.New()), identity.New())
		draft.AddConstraint(constraint.New(identity.New(), "missing", nil, false, ast.NewAbsolutePropertyReference(missing), constraint.SeverityHard, "", false))
		requireValidationIssue(t, draft.Validate(), ValidationIssueInvalidConstraintAssertion)
	})
}

func TestConstraintDependencyIndexIsCompleteDeterministicAndOwned(t *testing.T) {
	typeID, entityID := identity.New(), identity.New()
	aID, bID, cID := identity.New(), identity.New(), identity.New()
	draft := NewDraft()
	for _, definitionID := range []identity.UUID{aID, bID, cID} {
		draft.AddPropertyDefinition(schema.NewPropertyDefinition(definitionID, schema.EntityTypeOwner(typeID), identity.UUID{}, false, "flag", value.Bool, unit.Unit{}, false, schema.TemporalScalar, schema.InputComputation()))
	}
	draft.AddEntityType(schema.NewEntityType(typeID, "item", nil, []identity.UUID{aID, bID, cID}))
	draft.AddEntity(semantic.NewEntity(entityID, typeID, "", false))
	a := address.NewPropertyAddress(address.EntityRef(entityID), aID)
	b := address.NewPropertyAddress(address.EntityRef(entityID), bID)
	c := address.NewPropertyAddress(address.EntityRef(entityID), cID)
	constraintID := identity.New()
	reusedID := identity.New()
	draft.AddConstraint(constraint.New(constraintID, "indexed", ast.NewAbsolutePropertyReference(a), true, ast.And(ast.NewAbsolutePropertyReference(b), ast.NewAbsolutePropertyReference(c)), constraint.SeverityHard, "", false))
	// A appears in both decision phases and B appears twice in the assertion.
	// The index must retain phase information while collapsing each reverse
	// relationship to one Constraint ID.
	draft.AddConstraint(constraint.New(reusedID, "reused", ast.NewAbsolutePropertyReference(a), true, ast.And(ast.NewAbsolutePropertyReference(a), ast.And(ast.NewAbsolutePropertyReference(b), ast.NewAbsolutePropertyReference(b))), constraint.SeverityHard, "", false))

	version, err := Commit(modelForConstraintTest(), draft, NewCommitOptions(identity.New(), "constraints", precision.New(6, precision.HalfEven)))
	if err != nil {
		t.Fatal(err)
	}
	dependencies, ok := version.ConstraintDependencies(constraintID)
	if !ok || !dependencies.IsValid() {
		t.Fatal("missing valid constraint dependency index")
	}
	if got := dependencies.Applicability(); len(got) != 1 || got[0] != a {
		t.Fatalf("applicability dependencies = %v", got)
	}
	if got := dependencies.Assertion(); len(got) != 2 || !containsAddress(got, b) || !containsAddress(got, c) {
		t.Fatalf("assertion dependencies = %v", got)
	}
	if got := dependencies.All(); len(got) != 3 || !containsAddress(got, a) || !containsAddress(got, b) || !containsAddress(got, c) {
		t.Fatalf("all dependencies = %v", got)
	}
	all := dependencies.All()
	all[0] = address.PropertyAddress{}
	if got, _ := version.ConstraintDependencies(constraintID); !got.IsValid() || len(got.All()) != 3 {
		t.Fatal("dependency getter leaked storage")
	}
	reused, ok := version.ConstraintDependencies(reusedID)
	if !ok || len(reused.Applicability()) != 1 || len(reused.Assertion()) != 2 || len(reused.All()) != 2 {
		t.Fatalf("reused dependency canonicalization failed: %#v", reused)
	}
	for _, test := range []struct {
		propertyAddress address.PropertyAddress
		want            []identity.UUID
	}{{a, []identity.UUID{constraintID, reusedID}}, {b, []identity.UUID{constraintID, reusedID}}, {c, []identity.UUID{constraintID}}} {
		ids := version.ConstraintIDsDependingOn(test.propertyAddress)
		if len(ids) != len(test.want) || !sameUUIDSet(ids, test.want) {
			t.Fatalf("reverse index for %v = %v", test.propertyAddress, ids)
		}
		if len(ids) > 1 && ids[0].String() >= ids[1].String() {
			t.Fatalf("reverse index is not canonically ordered: %v", ids)
		}
		ids[0] = identity.UUID{}
		if got := version.ConstraintIDsDependingOn(test.propertyAddress); len(got) != len(test.want) || !sameUUIDSet(got, test.want) {
			t.Fatal("reverse index leaked storage")
		}
	}
}

func containsAddress(values []address.PropertyAddress, target address.PropertyAddress) bool {
	for _, item := range values {
		if item == target {
			return true
		}
	}
	return false
}

func sameUUIDSet(left, right []identity.UUID) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[identity.UUID]struct{}, len(left))
	for _, item := range left {
		seen[item] = struct{}{}
	}
	for _, item := range right {
		if _, ok := seen[item]; !ok {
			return false
		}
	}
	return true
}

func modelForConstraintTest() Model { return NewModel(identity.New(), "constraints") }
