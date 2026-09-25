package model

import (
	"testing"

	"github.com/shmatovv/bcg-engine/v2/internal/domain/identity"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/schema"
	"github.com/shmatovv/bcg-engine/v2/internal/domain/semantic"
	domaintime "github.com/shmatovv/bcg-engine/v2/internal/domain/time"
	"github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

func TestDraft_ValueBindingsAreStoredWithoutLeakingSlice(
	t *testing.T,
) {
	draft := NewDraft()

	if !draft.IsEmpty() {
		t.Fatal("new draft must be empty")
	}

	binding := testBaseValueBinding()

	draft.AddValueBinding(binding)

	if draft.IsEmpty() {
		t.Fatal(
			"draft with value binding must not be empty",
		)
	}

	got := draft.ValueBindings()

	if len(got) != 1 ||
		!got[0].IsValid() {

		t.Fatalf(
			"unexpected value bindings: %#v",
			got,
		)
	}

	// Mutating the returned slice must not mutate Draft.
	got[0] = semantic.ValueBinding{}

	again := draft.ValueBindings()

	if len(again) != 1 ||
		!again[0].IsValid() {

		t.Fatal(
			"draft leaked internal value binding slice",
		)
	}
}

func TestModelVersion_ValueBindingsAreImmutableAtCollectionBoundary(
	t *testing.T,
) {
	source := []semantic.ValueBinding{
		testBaseValueBinding(),
	}

	version := newModelVersion(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		source,
	)

	// Mutating the caller-owned input slice must not mutate ModelVersion.
	source[0] = semantic.ValueBinding{}

	got := version.ValueBindings()

	if len(got) != 1 ||
		!got[0].IsValid() {

		t.Fatal(
			"ModelVersion retained caller-owned value binding slice",
		)
	}

	// Mutating a returned slice must also not mutate ModelVersion.
	got[0] = semantic.ValueBinding{}

	again := version.ValueBindings()

	if len(again) != 1 ||
		!again[0].IsValid() {

		t.Fatal(
			"ModelVersion leaked internal value binding slice",
		)
	}
}

func testBaseValueBinding() semantic.ValueBinding {
	target := semantic.TypePropertyTarget(
		schema.EntityTypeOwner(
			identity.New(),
		),
		identity.New(),
	)

	return semantic.NewValueBinding(
		target,
		semantic.DefinedBindingValue(
			value.NewTyped(
				value.NewInt64(42),
			),
		),
		nil,
		domaintime.EffectivePeriod{},
		false,
	)
}
