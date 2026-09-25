package evaluator

import (
	"github.com/shmatovv/bcg-engine/v2/internal/function"
	enginevalue "github.com/shmatovv/bcg-engine/v2/internal/types/value"
)

// FunctionRuntime is the narrow host-runtime capability used by FunctionCall.
// It is deliberately separate from Source so semantic Models never acquire a
// callback or registry identity.
type FunctionRuntime interface {
	Invoke(function.Requirement, []enginevalue.Typed) (enginevalue.Resolved, error)
}

type runtimeBoundSource struct {
	Source
	runtime FunctionRuntime
}

func (s runtimeBoundSource) FunctionRuntime() FunctionRuntime { return s.runtime }
func (s runtimeBoundSource) FunctionRequirement(ref function.Ref) (function.Requirement, bool) {
	lookup, ok := s.Source.(interface {
		FunctionRequirement(function.Ref) (function.Requirement, bool)
	})
	if !ok {
		return function.Requirement{}, false
	}
	return lookup.FunctionRequirement(ref)
}

// BindRuntime decorates one immutable semantic source with an immutable host
// runtime for one evaluation/session lifetime.
func BindRuntime(source Source, runtime FunctionRuntime) Source {
	if runtime == nil {
		return source
	}
	return runtimeBoundSource{Source: source, runtime: runtime}
}

func functionRuntimeFromSource(source Source) FunctionRuntime {
	if bound, ok := source.(interface{ FunctionRuntime() FunctionRuntime }); ok {
		return bound.FunctionRuntime()
	}
	return nil
}
