package bcg

import (
	"errors"
	"fmt"

	constraintcheck "github.com/shmatovv/bcg-engine/v2/internal/constraint/check"
	"github.com/shmatovv/bcg-engine/v2/internal/evaluator"
	formatbcg "github.com/shmatovv/bcg-engine/v2/internal/format/bcg"
	"github.com/shmatovv/bcg-engine/v2/internal/impact"
	"github.com/shmatovv/bcg-engine/v2/internal/model"
	resolvervalue "github.com/shmatovv/bcg-engine/v2/internal/resolver/value"
	ruleeval "github.com/shmatovv/bcg-engine/v2/internal/rule/eval"
)

var (
	// ErrMalformedArtifact identifies syntactically malformed or structurally
	// invalid .bcg input.
	ErrMalformedArtifact = formatbcg.ErrMalformedDocument
	// ErrUnsupportedFormat identifies an unsupported .bcg wire version.
	ErrUnsupportedFormat = formatbcg.ErrUnsupportedFormatVersion
	// ErrIncompatibleEngine identifies a .bcg artifact for another semantic
	// engine compatibility generation.
	ErrIncompatibleEngine = formatbcg.ErrUnsupportedEngineCompatibility
	// ErrWireSemantic identifies a syntactically valid document whose wire
	// values cannot represent BCG v1 semantics.
	ErrWireSemantic = formatbcg.ErrWireSemantic
	// ErrModelValidation identifies an artifact rejected by normal committed
	// model validation. Use errors.As with ValidationError for its issues.
	ErrModelValidation = formatbcg.ErrModelValidation
	// ErrScenarioValidation identifies an artifact Scenario that is invalid for
	// its embedded Model version.
	ErrScenarioValidation = formatbcg.ErrScenarioValidation
	// ErrInvalidArgument identifies an invalid public API value or zero facade.
	ErrInvalidArgument = errors.New("bcg: invalid argument")
	// ErrInvalidModel identifies an invalid public Model or Artifact wrapper.
	ErrInvalidModel = errors.New("bcg: invalid model")
)

// ValidationIssue is one machine-readable model validation problem.
type ValidationIssue struct {
	kind       string
	subjectID  ID
	hasSubject bool
	relatedID  ID
	hasRelated bool
	message    string
}

// Kind classifies the failed model invariant.
func (i ValidationIssue) Kind() string { return i.kind }

// SubjectID identifies the primary affected model object when available.
func (i ValidationIssue) SubjectID() (ID, bool) { return i.subjectID, i.hasSubject }

// RelatedID identifies the related model object when available.
func (i ValidationIssue) RelatedID() (ID, bool) { return i.relatedID, i.hasRelated }

// Message provides a human-readable diagnostic.
func (i ValidationIssue) Message() string { return i.message }

// ValidationError retains validation issues from a rejected artifact or Draft
// commit. It supports errors.Is for the underlying codec category and errors.As
// for this public type.
type ValidationError struct {
	cause  error
	issues []ValidationIssue
}

// ValidationReport is the deterministic result of validating a Draft.
// A report with no issues is valid. It owns its issue slice.
type ValidationReport struct{ issues []ValidationIssue }

// OK reports whether validation found no issues.
func (r ValidationReport) OK() bool { return len(r.issues) == 0 }

// Len returns the number of validation issues.
func (r ValidationReport) Len() int { return len(r.issues) }

// Issues returns an owned copy of the issues in deterministic order.
func (r ValidationReport) Issues() []ValidationIssue {
	return append([]ValidationIssue(nil), r.issues...)
}

// Error implements error.
func (e *ValidationError) Error() string {
	if e == nil {
		return "bcg: validation error"
	}
	return fmt.Sprintf("bcg: model validation failed (%d issues): %v", len(e.issues), e.cause)
}

// Unwrap preserves the underlying format or validation category for errors.Is.
func (e *ValidationError) Unwrap() error { return e.cause }

// Issues returns an owned deterministic copy.
func (e *ValidationError) Issues() []ValidationIssue {
	if e == nil {
		return nil
	}
	return append([]ValidationIssue(nil), e.issues...)
}

// EngineError is an operation failure. Defined, Null, and Undefined results
// never produce an EngineError.
type EngineError struct {
	kind  string
	cause error
}

// Error implements error.
func (e *EngineError) Error() string {
	if e == nil {
		return "bcg: engine error"
	}
	return fmt.Sprintf("bcg: %s: %v", e.kind, e.cause)
}

// Unwrap preserves the underlying core error for diagnostics.
func (e *EngineError) Unwrap() error { return e.cause }

// Kind is the stable semantic failure category.
func (e *EngineError) Kind() string {
	if e == nil {
		return ""
	}
	return e.kind
}

func publicError(err error) error {
	if err == nil {
		return nil
	}
	var report model.ValidationReport
	if errors.As(err, &report) {
		return &ValidationError{cause: err, issues: validationIssuesFromInternal(report.Issues())}
	}
	var evaluation evaluator.Error
	if errors.As(err, &evaluation) {
		return &EngineError{kind: evaluation.Kind().String(), cause: err}
	}
	var analyzed impact.Error
	if errors.As(err, &analyzed) {
		return &EngineError{kind: analyzed.Kind().String(), cause: err}
	}
	var checked constraintcheck.Error
	if errors.As(err, &checked) {
		return &EngineError{kind: checked.Kind().String(), cause: err}
	}
	var ruled ruleeval.Error
	if errors.As(err, &ruled) {
		return &EngineError{kind: ruled.Kind().String(), cause: err}
	}
	var resolved resolvervalue.Error
	if errors.As(err, &resolved) {
		return &EngineError{kind: resolved.Kind().String(), cause: err}
	}
	return err
}

func validationReportFromInternal(report model.ValidationReport) ValidationReport {
	return ValidationReport{issues: validationIssuesFromInternal(report.Issues())}
}

func validationIssuesFromInternal(issues []model.ValidationIssue) []ValidationIssue {
	public := make([]ValidationIssue, len(issues))
	for index, issue := range issues {
		public[index] = ValidationIssue{kind: issue.Kind().String(), message: issue.Message()}
		if subject, ok := issue.SubjectID(); ok {
			public[index].subjectID, public[index].hasSubject = idFromInternal(subject), true
		}
		if related, ok := issue.RelatedID(); ok {
			public[index].relatedID, public[index].hasRelated = idFromInternal(related), true
		}
	}
	return public
}
