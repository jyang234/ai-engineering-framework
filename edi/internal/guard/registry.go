package guard

import "fmt"

// Registry holds an ordered list of policies.
// Order matters: policies are evaluated first-to-last, and a Block
// from any policy short-circuits further evaluation.
type Registry struct {
	policies []interface{}
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a policy to the end of the evaluation order.
// Panics if the value implements none of the four policy interfaces.
// This is a programmer error caught at startup, not a runtime condition.
func (r *Registry) Register(policy interface{}) {
	_, a := policy.(PreToolUsePolicy)
	_, b := policy.(PostToolUsePolicy)
	_, c := policy.(PostToolUseFailurePolicy)
	_, d := policy.(PreCompactPolicy)
	if !a && !b && !c && !d {
		panic(fmt.Sprintf("guard: registered policy %T implements none of "+
			"PreToolUsePolicy, PostToolUsePolicy, PostToolUseFailurePolicy, "+
			"PreCompactPolicy", policy))
	}
	r.policies = append(r.policies, policy)
}

// PreToolUsePolicies returns all policies that implement PreToolUsePolicy.
func (r *Registry) PreToolUsePolicies() []PreToolUsePolicy {
	var result []PreToolUsePolicy
	for _, p := range r.policies {
		if ptu, ok := p.(PreToolUsePolicy); ok {
			result = append(result, ptu)
		}
	}
	return result
}

// PostToolUsePolicies returns all policies that implement PostToolUsePolicy.
func (r *Registry) PostToolUsePolicies() []PostToolUsePolicy {
	var result []PostToolUsePolicy
	for _, p := range r.policies {
		if pp, ok := p.(PostToolUsePolicy); ok {
			result = append(result, pp)
		}
	}
	return result
}

// PostToolUseFailurePolicies returns all policies that implement PostToolUseFailurePolicy.
func (r *Registry) PostToolUseFailurePolicies() []PostToolUseFailurePolicy {
	var result []PostToolUseFailurePolicy
	for _, p := range r.policies {
		if pp, ok := p.(PostToolUseFailurePolicy); ok {
			result = append(result, pp)
		}
	}
	return result
}

// PreCompactPolicies returns all policies that implement PreCompactPolicy.
func (r *Registry) PreCompactPolicies() []PreCompactPolicy {
	var result []PreCompactPolicy
	for _, p := range r.policies {
		if pp, ok := p.(PreCompactPolicy); ok {
			result = append(result, pp)
		}
	}
	return result
}
