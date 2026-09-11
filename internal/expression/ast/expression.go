package ast

// Expression is a node of the declarative expression AST.
//
// Implementations must be immutable and side-effect-free.
//
// The unexported marker method makes the set of expression nodes
// controlled by this package.
type Expression interface {
	expressionNode()
}
