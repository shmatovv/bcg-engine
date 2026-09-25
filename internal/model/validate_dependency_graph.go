package model

import (
	"fmt"
	"strings"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/address"
	"github.com/shmatovv/bcg-engine/v2/internal/graph/dependency"
)

// validateDependencyGraph constructs the concrete value dependency graph and
// rejects every cyclic strongly connected component.
//
// Dependency extraction runs after structural/reference/expression validation.
//
// If dependency.Build fails while the preceding validation passes have already
// found model errors, no extra graph diagnostic is emitted. The build failure
// is expected to be a consequence of an already-invalid Draft.
//
// If every preceding pass succeeded but dependency.Build still fails, the
// failure is reported explicitly. Such a state means either:
//
//   - validation missed a graph-relevant invariant;
//   - dependency extraction does not support an otherwise valid construct;
//   - model internals are inconsistent.
//
// A committed ModelVersion must never be produced in that state.
func (d *Draft) validateDependencyGraph(
	report *ValidationReport,
	indexes validationIndexes,
) {
	if report == nil {
		panic("model: nil validation report")
	}

	source :=
		newDraftDependencySource(
			d,
			indexes,
		)

	graph, err :=
		dependency.Build(source)

	if err != nil {
		if report.OK() {
			report.append(
				newValidationIssue(
					ValidationIssueDependencyGraphBuildFailure,
					fmt.Sprintf(
						"concrete dependency graph could not be built: %v",
						err,
					),
				),
			)
		}

		return
	}

	cycles :=
		dependency.FindCycles(graph)

	for _, cycle := range cycles {

		vertices :=
			cycle.Vertices()

		if len(vertices) == 0 {
			continue
		}

		first := vertices[0]

		report.append(
			newRelatedValidationIssue(
				ValidationIssueCycle,
				first.Object().ID(),
				first.DefinitionID(),
				formatDependencyCycle(
					vertices,
				),
			),
		)
	}
}

func formatDependencyCycle(
	vertices []address.PropertyAddress,
) string {
	if len(vertices) == 0 {
		return "value dependency cycle detected"
	}

	var builder strings.Builder

	builder.WriteString(
		"value dependency cycle detected: ",
	)

	for index, vertex := range vertices {

		if index > 0 {
			builder.WriteString(
				" <-> ",
			)
		}

		builder.WriteString(
			vertex.String(),
		)
	}

	if len(vertices) == 1 {
		builder.WriteString(
			" -> self",
		)
	}

	return builder.String()
}
