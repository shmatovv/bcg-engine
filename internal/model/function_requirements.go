package model

import (
	"sort"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/typing"
	"github.com/shmatovv/bcg-engine/v2/internal/function"
)

// validateFunctionRequirements makes the declaration list a typechecking aid,
// not a second source of truth. Every call must resolve exactly one valid
// requirement; commit then derives its persisted requirement list from calls.
func (d *Draft) validateFunctionRequirements(report *ValidationReport, indexes validationIndexes) {
	seen := make(map[string]struct{}, len(d.functionRequirements))
	for _, requirement := range d.functionRequirements {
		if !requirement.IsValid() {
			report.append(newValidationIssue(ValidationIssueInvalidFunctionRequirement, "function requirement is structurally invalid"))
			continue
		}
		for _, functionType := range append(requirement.Signature().Parameters(), requirement.Signature().Result()) {
			if !functionTypeReferencesKnownEnum(functionType, indexes) {
				report.append(newValidationIssue(ValidationIssueInvalidFunctionRequirement, "function requirement references an unknown EnumDefinition"))
			}
		}
		key := requirement.Ref().Key()
		if _, exists := seen[key]; exists {
			report.append(newValidationIssue(ValidationIssueDuplicateFunctionRequirement, "duplicate function requirement "+requirement.Ref().String()))
			continue
		}
		seen[key] = struct{}{}
	}
	for _, ref := range d.calledFunctionRefs() {
		if _, exists := seen[ref.Key()]; !exists {
			report.append(newValidationIssue(ValidationIssueUnknownFunctionRequirement, "function call has no matching requirement: "+ref.String()))
		}
	}
}

func functionTypeReferencesKnownEnum(functionType typing.Type, indexes validationIndexes) bool {
	enumDefinitionID, hasEnum := functionType.EnumDefinitionID()
	if !hasEnum {
		return true
	}
	_, exists := indexes.enumDefinitionByID[enumDefinitionID]
	return exists
}

func (d *Draft) calledFunctionRefs() []function.Ref {
	refs := make(map[string]function.Ref)
	add := func(expression ast.Expression) { collectFunctionRefs(expression, refs) }
	for _, definition := range d.propertyDefinitions {
		if definition.IsValid() && definition.Computation().Kind() == schema.ComputationDerived {
			if expression, ok := definition.Computation().Expression(); ok {
				add(expression)
			}
		}
	}
	for _, relation := range d.relations {
		if expression, ok := relation.Applicability(); ok {
			add(expression)
		}
	}
	for _, constraint := range d.constraints {
		if expression, ok := constraint.Applicability(); ok {
			add(expression)
		}
		add(constraint.Assertion())
	}
	for _, rule := range d.rules {
		add(rule.Condition())
	}
	for _, invariant := range d.invariants {
		if expression, ok := invariant.Applicability(); ok {
			add(expression)
		}
		add(invariant.Assertion())
	}
	result := make([]function.Ref, 0, len(refs))
	for _, ref := range refs {
		result = append(result, ref)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key() < result[j].Key() })
	return result
}

func collectFunctionRefs(expression ast.Expression, refs map[string]function.Ref) {
	if expression == nil {
		return
	}
	switch node := expression.(type) {
	case ast.FunctionCall:
		if node.IsValid() {
			refs[node.Ref().Key()] = node.Ref()
			for _, arg := range node.Arguments() {
				collectFunctionRefs(arg, refs)
			}
		}
	case ast.Arithmetic:
		collectFunctionRefs(node.Left(), refs)
		collectFunctionRefs(node.Right(), refs)
	case ast.Negate:
		collectFunctionRefs(node.Operand(), refs)
	case ast.Comparison:
		collectFunctionRefs(node.Left(), refs)
		collectFunctionRefs(node.Right(), refs)
	case ast.Logical:
		collectFunctionRefs(node.Left(), refs)
		collectFunctionRefs(node.Right(), refs)
	case ast.Not:
		collectFunctionRefs(node.Operand(), refs)
	case ast.If:
		collectFunctionRefs(node.Condition(), refs)
		collectFunctionRefs(node.Then(), refs)
		collectFunctionRefs(node.Else(), refs)
	case ast.IsNull:
		collectFunctionRefs(node.Operand(), refs)
	case ast.Coalesce:
		collectFunctionRefs(node.Primary(), refs)
		collectFunctionRefs(node.Fallback(), refs)
	case ast.Aggregate:
		if filter, ok := node.Filter(); ok {
			collectFunctionRefs(filter, refs)
		}
		if mapped, ok := node.Map(); ok {
			collectFunctionRefs(mapped, refs)
		}
	case ast.Unary:
		collectFunctionRefs(node.Operand(), refs)
	case ast.Binary:
		collectFunctionRefs(node.Left(), refs)
		collectFunctionRefs(node.Right(), refs)
	case ast.Variadic:
		for _, item := range node.Operands() {
			collectFunctionRefs(item, refs)
		}
	}
}

func canonicalFunctionRequirements(d *Draft) []function.Requirement {
	byRef := make(map[string]function.Requirement, len(d.functionRequirements))
	for _, requirement := range d.functionRequirements {
		if requirement.IsValid() {
			byRef[requirement.Ref().Key()] = requirement
		}
	}
	refs := d.calledFunctionRefs()
	result := make([]function.Requirement, 0, len(refs))
	for _, ref := range refs {
		if requirement, ok := byRef[ref.Key()]; ok {
			result = append(result, requirement)
		}
	}
	return result
}
