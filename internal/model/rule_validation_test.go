package model

import (
	"testing"

	"github.com/shmatovv/bcg-engine/internal/domain/address"
	"github.com/shmatovv/bcg-engine/internal/domain/identity"
	domainrule "github.com/shmatovv/bcg-engine/internal/domain/rule"
	"github.com/shmatovv/bcg-engine/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/internal/domain/semantic"
	"github.com/shmatovv/bcg-engine/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/internal/types/precision"
	"github.com/shmatovv/bcg-engine/internal/types/unit"
	"github.com/shmatovv/bcg-engine/internal/types/value"
)

func TestDraftValidateRuleBooleanContractsAndReferences(t *testing.T) {
	t.Run("structurally invalid rule", func(t *testing.T) {
		draft := NewDraft()
		draft.AddRule(domainrule.Rule{})
		requireValidationIssue(t, draft.Validate(), ValidationIssueInvalidRule)
	})

	t.Run("non Boolean scalar condition", func(t *testing.T) {
		draft := NewDraft()
		draft.AddRule(domainrule.New(identity.New(), "bad", intConstant(1)))
		requireValidationIssue(t, draft.Validate(), ValidationIssueInvalidRuleCondition)
	})

	t.Run("Series Bool does not imply all or any", func(t *testing.T) {
		fixture := newBaseValueFixture(t, schema.InputComputation(), schema.TemporalSeries, identity.UUID{}, false, value.Bool, unit.Unit{}, false)
		target := address.NewPropertyAddress(address.EntityRef(fixture.entityID), fixture.definitionID)
		fixture.draft.AddRule(domainrule.New(identity.New(), "series", ast.NewAbsolutePropertyReference(target)))
		requireValidationIssue(t, fixture.draft.Validate(), ValidationIssueInvalidRuleCondition)
	})

	t.Run("invalid absolute property reference", func(t *testing.T) {
		draft := NewDraft()
		missing := address.NewPropertyAddress(address.EntityRef(identity.New()), identity.New())
		draft.AddRule(domainrule.New(identity.New(), "missing", ast.NewAbsolutePropertyReference(missing)))
		requireValidationIssue(t, draft.Validate(), ValidationIssueInvalidRuleCondition)
	})

	t.Run("self bound expression has no implicit standalone self", func(t *testing.T) {
		draft := NewDraft()
		draft.AddRule(domainrule.New(identity.New(), "self", ast.NewSelfPropertyReference(identity.New())))
		requireValidationIssue(t, draft.Validate(), ValidationIssueInvalidRuleCondition)
	})

	t.Run("malformed expression", func(t *testing.T) {
		draft := NewDraft()
		draft.AddRule(domainrule.New(identity.New(), "malformed", ast.Constant{}))
		requireValidationIssue(t, draft.Validate(), ValidationIssueInvalidRuleCondition)
	})

	t.Run("duplicate ID", func(t *testing.T) {
		draft := NewDraft()
		id := identity.New()
		draft.AddRule(domainrule.New(id, "one", boolConstant(true)))
		draft.AddRule(domainrule.New(id, "two", boolConstant(false)))
		requireValidationIssue(t, draft.Validate(), ValidationIssueDuplicateRuleID)
	})
}

func TestRuleDependencyIndexIsCompleteDeterministicAndOwned(t *testing.T) {
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
	ruleID, duplicateID := identity.New(), identity.New()
	draft.AddRule(domainrule.New(ruleID, "all", ast.And(ast.NewAbsolutePropertyReference(a), ast.And(ast.NewAbsolutePropertyReference(b), ast.NewAbsolutePropertyReference(c)))))
	draft.AddRule(domainrule.New(duplicateID, "duplicate", ast.And(ast.NewAbsolutePropertyReference(a), ast.NewAbsolutePropertyReference(a))))

	version, err := Commit(modelForConstraintTest(), draft, NewCommitOptions(identity.New(), "rules", precision.New(6, precision.HalfEven)))
	if err != nil {
		t.Fatal(err)
	}
	dependencies, ok := version.RuleDependencies(ruleID)
	if !ok || !dependencies.IsValid() {
		t.Fatal("missing valid Rule dependency index")
	}
	if got := dependencies.Condition(); len(got) != 3 || !containsAddress(got, a) || !containsAddress(got, b) || !containsAddress(got, c) {
		t.Fatalf("condition dependencies = %v", got)
	}
	owned := dependencies.Condition()
	owned[0] = address.PropertyAddress{}
	if got, _ := version.RuleDependencies(ruleID); len(got.Condition()) != 3 {
		t.Fatal("RuleDependencies exposes mutable storage")
	}
	if got := version.RuleIDsDependingOn(a); len(got) != 2 || got[0] == got[1] {
		t.Fatalf("reverse dependencies = %v", got)
	}
	reverse := version.RuleIDsDependingOn(a)
	reverse[0] = identity.UUID{}
	if got := version.RuleIDsDependingOn(a); len(got) != 2 || got[0].IsZero() {
		t.Fatal("Rule reverse index exposes mutable storage")
	}
	if got := version.RuleIDsDependingOn(b); len(got) != 1 || got[0] != ruleID {
		t.Fatalf("B reverse dependencies = %v", got)
	}
}

func TestRuleMetadataIsDefensivelyOwnedAcrossCommittedSnapshot(t *testing.T) {
	metadata := map[string]string{"source": "test"}
	modelRule := domainrule.NewWithMetadata(identity.New(), "metadata", boolConstant(true), metadata)
	metadata["source"] = "caller mutation"
	if got := modelRule.Metadata()["source"]; got != "test" {
		t.Fatalf("constructor retained metadata backing map: %q", got)
	}
	draft := NewDraft()
	draft.AddRule(modelRule)
	version, err := Commit(modelForConstraintTest(), draft, NewCommitOptions(identity.New(), "metadata", precision.New(6, precision.HalfEven)))
	if err != nil {
		t.Fatal(err)
	}
	rules := version.Rules()
	mutated := rules[0].Metadata()
	mutated["source"] = "result mutation"
	stored, ok := version.Rule(modelRule.ID())
	if !ok || stored.Metadata()["source"] != "test" {
		t.Fatal("committed Rule metadata is mutable through a getter")
	}
}
