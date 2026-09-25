package dependency

import (
	"fmt"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/expression/ast"
	"github.com/shmatovv/bcg-engine/v2/internal/function"
)

// StaticKind identifies one syntactic dependency requirement. This is the
// single AST interpretation shared by concrete graph construction and schema
// planning. Resolution of a requirement to a concrete runtime address remains
// source-specific.
type StaticKind uint8

const (
	StaticSelfProperty StaticKind = iota + 1
	StaticAbsoluteProperty
	StaticBoundProperty
	StaticBoundContract
	StaticTraversal
	StaticFunction
)

// StaticPhase distinguishes aggregate filter and mapped-item reads.
type StaticPhase uint8

const (
	StaticPhaseOrdinary StaticPhase = iota
	StaticPhaseFilter
	StaticPhaseMap
)

// StaticRequirement is an immutable AST-level fact. TraversalID identifies a
// traversal declaration. ParentTraversalID identifies the lexical aggregate
// whose item context resolves this requirement; zero is the expression root.
type StaticRequirement struct {
	kind              StaticKind
	phase             StaticPhase
	traversalID       uint32
	parentTraversalID uint32
	definitionID      identity.UUID
	contractID        identity.UUID
	address           address.PropertyAddress
	boundTarget       ast.BoundTarget
	traversal         ast.TraversalSource
	function          function.Ref
}

func (r StaticRequirement) Kind() StaticKind    { return r.kind }
func (r StaticRequirement) Phase() StaticPhase  { return r.phase }
func (r StaticRequirement) TraversalID() uint32 { return r.traversalID }
func (r StaticRequirement) ParentTraversalID() uint32 {
	if r.kind == StaticTraversal {
		return r.parentTraversalID
	}
	return r.traversalID
}
func (r StaticRequirement) DefinitionID() identity.UUID { return r.definitionID }
func (r StaticRequirement) ContractID() identity.UUID   { return r.contractID }
func (r StaticRequirement) Address() (address.PropertyAddress, bool) {
	return r.address, r.kind == StaticAbsoluteProperty
}
func (r StaticRequirement) BoundTarget() ast.BoundTarget { return r.boundTarget }
func (r StaticRequirement) Traversal() (ast.TraversalSource, bool) {
	return r.traversal, r.kind == StaticTraversal
}
func (r StaticRequirement) Function() (function.Ref, bool) {
	return r.function, r.kind == StaticFunction
}

// AnalyzeStatic returns AST-level dependencies in deterministic pre-order.
// It deliberately does not resolve concrete graph objects and therefore never
// invents Dataset identities.
func AnalyzeStatic(expression ast.Expression) ([]StaticRequirement, error) {
	if expression == nil {
		return nil, fmt.Errorf("dependency: invalid static expression")
	}
	depth, valid := ast.Depth(expression)
	if !valid || depth > ast.MaxExpressionDepth {
		return nil, fmt.Errorf("dependency: invalid static expression")
	}
	a := staticAnalyzer{}
	if err := a.walk(expression, StaticPhaseOrdinary, 0); err != nil {
		return nil, err
	}
	return a.out, nil
}

type staticAnalyzer struct {
	out  []StaticRequirement
	next uint32
}

func (a *staticAnalyzer) walk(e ast.Expression, phase StaticPhase, traversalID uint32) error {
	if e == nil {
		return fmt.Errorf("dependency: invalid static expression")
	}
	switch x := e.(type) {
	case ast.Constant:
		return nil
	case ast.SelfPropertyReference:
		a.out = append(a.out, StaticRequirement{kind: StaticSelfProperty, phase: phase, traversalID: traversalID, definitionID: x.DefinitionID()})
		return nil
	case ast.AbsolutePropertyReference:
		a.out = append(a.out, StaticRequirement{kind: StaticAbsoluteProperty, phase: phase, traversalID: traversalID, address: x.Address(), definitionID: x.Address().DefinitionID()})
		return nil
	case ast.BoundPropertyDefinitionReference:
		a.out = append(a.out, StaticRequirement{kind: StaticBoundProperty, phase: phase, traversalID: traversalID, definitionID: x.DefinitionID(), boundTarget: x.Target()})
		return nil
	case ast.BoundContractPropertyReference:
		a.out = append(a.out, StaticRequirement{kind: StaticBoundContract, phase: phase, traversalID: traversalID, contractID: x.ContractID(), boundTarget: x.Target()})
		return nil
	case ast.Aggregate:
		a.next++
		id := a.next
		a.out = append(a.out, StaticRequirement{kind: StaticTraversal, phase: phase, traversalID: id, parentTraversalID: traversalID, traversal: x.Source()})
		if f, ok := x.Filter(); ok {
			if err := a.walk(f, StaticPhaseFilter, id); err != nil {
				return err
			}
		}
		if m, ok := x.Map(); ok {
			if err := a.walk(m, StaticPhaseMap, id); err != nil {
				return err
			}
		}
		return nil
	case ast.FunctionCall:
		a.out = append(a.out, StaticRequirement{kind: StaticFunction, phase: phase, traversalID: traversalID, function: x.Ref()})
		for _, c := range x.Arguments() {
			if err := a.walk(c, phase, traversalID); err != nil {
				return err
			}
		}
		return nil
	case ast.Arithmetic:
		return a.walk2(x.Left(), x.Right(), phase, traversalID)
	case ast.Negate:
		return a.walk(x.Operand(), phase, traversalID)
	case ast.Comparison:
		return a.walk2(x.Left(), x.Right(), phase, traversalID)
	case ast.Logical:
		return a.walk2(x.Left(), x.Right(), phase, traversalID)
	case ast.Not:
		return a.walk(x.Operand(), phase, traversalID)
	case ast.If:
		if err := a.walk(x.Condition(), phase, traversalID); err != nil {
			return err
		}
		if err := a.walk(x.Then(), phase, traversalID); err != nil {
			return err
		}
		return a.walk(x.Else(), phase, traversalID)
	case ast.IsNull:
		return a.walk(x.Operand(), phase, traversalID)
	case ast.Coalesce:
		return a.walk2(x.Primary(), x.Fallback(), phase, traversalID)
	case ast.Unary:
		return a.walk(x.Operand(), phase, traversalID)
	case ast.Binary:
		return a.walk2(x.Left(), x.Right(), phase, traversalID)
	case ast.Variadic:
		for _, c := range x.Operands() {
			if err := a.walk(c, phase, traversalID); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("dependency: unsupported static expression")
	}
}
func (a *staticAnalyzer) walk2(l, r ast.Expression, p StaticPhase, id uint32) error {
	if err := a.walk(l, p, id); err != nil {
		return err
	}
	return a.walk(r, p, id)
}
