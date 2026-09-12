package bcg

import (
	constraintcheck "github.com/shmatovv/bcg-engine/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/internal/constraint/feasibility"
	"github.com/shmatovv/bcg-engine/internal/domain/constraint"
)

// ConstraintState is the semantic outcome of a declarative Constraint.
type ConstraintState string

const (
	ConstraintSatisfied     ConstraintState = "Satisfied"
	ConstraintViolated      ConstraintState = "Violated"
	ConstraintNotApplicable ConstraintState = "NotApplicable"
	ConstraintUndefined     ConstraintState = "Undefined"
)

// ConstraintSeverity is a model-declared business severity.
type ConstraintSeverity string

const (
	SeverityInfo    ConstraintSeverity = "Info"
	SeverityWarning ConstraintSeverity = "Warning"
	SeverityError   ConstraintSeverity = "Error"
	SeverityHard    ConstraintSeverity = "Hard"
)

// ConstraintInfo is immutable metadata for one model Constraint.
type ConstraintInfo struct {
	id         ID
	name       string
	severity   ConstraintSeverity
	message    string
	hasMessage bool
	definition Constraint
}

// ID returns the Constraint UUID.
func (c ConstraintInfo) ID() ID { return c.id }

// Name returns the Constraint display name.
func (c ConstraintInfo) Name() string { return c.name }

// Severity returns the declared business severity.
func (c ConstraintInfo) Severity() ConstraintSeverity { return c.severity }

// Message returns the optional caller-provided diagnostic message.
func (c ConstraintInfo) Message() (string, bool) { return c.message, c.hasMessage }

// Definition returns the complete immutable Constraint declaration, including
// its assertion and optional applicability expression.
func (c ConstraintInfo) Definition() Constraint { return c.definition }

// ConstraintResult retains only applicability/assertion results actually evaluated.
type ConstraintResult struct {
	constraint       ConstraintInfo
	state            ConstraintState
	applicability    Result
	hasApplicability bool
	assertion        Result
	hasAssertion     bool
}

// Constraint returns the checked Constraint metadata.
func (r ConstraintResult) Constraint() ConstraintInfo { return r.constraint }

// State returns the Constraint semantic outcome.
func (r ConstraintResult) State() ConstraintState { return r.state }

// Applicability returns the actual applicability evaluation when evaluated.
func (r ConstraintResult) Applicability() (Result, bool) { return r.applicability, r.hasApplicability }

// Assertion returns the actual assertion evaluation when evaluated.
func (r ConstraintResult) Assertion() (Result, bool) { return r.assertion, r.hasAssertion }

// ConstraintReport contains every evaluated Constraint in deterministic order.
type ConstraintReport struct{ results []ConstraintResult }

// Results returns an owned deterministic copy of every Constraint result.
func (r ConstraintReport) Results() []ConstraintResult {
	return append([]ConstraintResult(nil), r.results...)
}

// Len returns the number of reported Constraints.
func (r ConstraintReport) Len() int { return len(r.results) }

func constraintInfoFromInternal(value constraint.Constraint) ConstraintInfo {
	message, hasMessage := value.Message()
	return ConstraintInfo{id: idFromInternal(value.ID()), name: value.Name(), severity: severityFromInternal(value.Severity()), message: message, hasMessage: hasMessage, definition: constraintFromInternal(value)}
}
func severityFromInternal(value constraint.Severity) ConstraintSeverity {
	switch value {
	case constraint.SeverityInfo:
		return SeverityInfo
	case constraint.SeverityWarning:
		return SeverityWarning
	case constraint.SeverityError:
		return SeverityError
	case constraint.SeverityHard:
		return SeverityHard
	default:
		return ""
	}
}
func constraintStateFromInternal(value constraintcheck.State) ConstraintState {
	switch value {
	case constraintcheck.StateSatisfied:
		return ConstraintSatisfied
	case constraintcheck.StateViolated:
		return ConstraintViolated
	case constraintcheck.StateNotApplicable:
		return ConstraintNotApplicable
	case constraintcheck.StateUndefined:
		return ConstraintUndefined
	default:
		return ""
	}
}
func constraintResultFromInternal(value constraintcheck.Result) ConstraintResult {
	result := ConstraintResult{constraint: constraintInfoFromInternal(value.Constraint()), state: constraintStateFromInternal(value.State())}
	if applicability, ok := value.Applicability(); ok {
		result.applicability, result.hasApplicability = resultFromInternal(applicability), true
	}
	if assertion, ok := value.Assertion(); ok {
		result.assertion, result.hasAssertion = resultFromInternal(assertion), true
	}
	return result
}
func constraintReportFromInternal(value constraintcheck.Report) ConstraintReport {
	items := value.Results()
	result := ConstraintReport{results: make([]ConstraintResult, len(items))}
	for i, item := range items {
		result.results[i] = constraintResultFromInternal(item)
	}
	return result
}

// FeasibilityState is derived solely from Hard Constraint outcomes.
type FeasibilityState string

const (
	Feasible     FeasibilityState = "Feasible"
	Infeasible   FeasibilityState = "Infeasible"
	Undetermined FeasibilityState = "Undetermined"
)

// FeasibilityResult is a complete Constraint report and its Hard-only conclusion.
type FeasibilityResult struct {
	state                       FeasibilityState
	report                      ConstraintReport
	violatedHard, undefinedHard []ID
}

// State returns the Hard-Constraint feasibility conclusion.
func (r FeasibilityResult) State() FeasibilityState { return r.state }

// Report returns all checked Constraints, including non-Hard ones.
func (r FeasibilityResult) Report() ConstraintReport { return r.report }

// ViolatedHard returns an owned deterministic list of violated Hard Constraints.
func (r FeasibilityResult) ViolatedHard() []ID { return append([]ID(nil), r.violatedHard...) }

// UndefinedHard returns an owned deterministic list of Undefined Hard Constraints.
func (r FeasibilityResult) UndefinedHard() []ID { return append([]ID(nil), r.undefinedHard...) }
func feasibilityResultFromInternal(value feasibility.Result) FeasibilityResult {
	result := FeasibilityResult{state: feasibilityStateFromInternal(value.State()), report: constraintReportFromInternal(value.Report())}
	for _, id := range value.ViolatedHard() {
		result.violatedHard = append(result.violatedHard, idFromInternal(id))
	}
	for _, id := range value.UndefinedHard() {
		result.undefinedHard = append(result.undefinedHard, idFromInternal(id))
	}
	return result
}
func feasibilityStateFromInternal(value feasibility.State) FeasibilityState {
	switch value {
	case feasibility.StateFeasible:
		return Feasible
	case feasibility.StateInfeasible:
		return Infeasible
	case feasibility.StateUndetermined:
		return Undetermined
	default:
		return ""
	}
}
