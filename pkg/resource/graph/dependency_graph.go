// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package graph

import (
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy/providers"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// DependencyGraph represents a dependency graph encoded within a resource snapshot.
type DependencyGraph struct {
	index      map[*resource.State]int // A mapping of resource pointers to indexes within the snapshot
	resources  []*resource.State       // The list of resources, obtained from the snapshot
	childrenOf map[resource.URN][]int  // Pre-computed map of transitive children for each resource
}

// DependingOn returns a slice containing all resources that directly or indirectly
// depend upon the given resource. The returned slice is guaranteed to be in topological
// order with respect to the snapshot dependency graph.
//
// The time complexity of DependingOn is linear with respect to the number of resources.
func (dg *DependencyGraph) DependingOn(res *resource.State,
	ignore map[resource.URN]bool, includeChildren bool) []*resource.State {
	// This implementation relies on the detail that snapshots are stored in a valid
	// topological order.
	var dependents []*resource.State
	dependentSet := make(map[resource.URN]bool)

	cursorIndex, ok := dg.index[res]
	contract.Assert(ok)
	dependentSet[res.URN] = true

	isDependent := func(candidate *resource.State) bool {
		if ignore[candidate.URN] {
			return false
		}
		if includeChildren && candidate.Parent == res.URN {
			return true
		}
		for _, dependency := range candidate.Dependencies {
			if dependentSet[dependency] {
				return true
			}
		}
		if candidate.Provider != "" {
			ref, err := providers.ParseReference(candidate.Provider)
			contract.Assert(err == nil)
			if dependentSet[ref.URN()] {
				return true
			}
		}
		return false
	}

	// The dependency graph encoded directly within the snapshot is the reverse of
	// the graph that we actually want to operate upon. Edges in the snapshot graph
	// originate in a resource and go to that resource's dependencies.
	//
	// The `DependingOn` is simpler when operating on the reverse of the snapshot graph,
	// where edges originate in a resource and go to resources that depend on that resource.
	// In this graph, `DependingOn` for a resource is the set of resources that are reachable from the
	// given resource.
	//
	// To accomplish this without building up an entire graph data structure, we'll do a linear
	// scan of the resource list starting at the requested resource and ending at the end of
	// the list. All resources that depend directly or indirectly on `res` are prepended
	// onto `dependents`.
	for i := cursorIndex + 1; i < len(dg.resources); i++ {
		candidate := dg.resources[i]
		if isDependent(candidate) {
			dependents = append(dependents, candidate)
			dependentSet[candidate.URN] = true
		}
	}

	return dependents
}

// DependenciesOf returns a ResourceSet of resources upon which the given resource depends. The resource's parent is
// included in the returned set.
func (dg *DependencyGraph) DependenciesOf(res *resource.State) ResourceSet {
	set := make(ResourceSet)

	dependentUrns := make(map[resource.URN]bool)
	for _, dep := range res.Dependencies {
		dependentUrns[dep] = true
	}

	if res.Provider != "" {
		ref, err := providers.ParseReference(res.Provider)
		contract.Assert(err == nil)
		dependentUrns[ref.URN()] = true
	}

	cursorIndex, ok := dg.index[res]
	contract.Assert(ok)
	for i := cursorIndex - 1; i >= 0; i-- {
		candidate := dg.resources[i]
		// Include all resources that are dependencies of the resource
		if dependentUrns[candidate.URN] {
			set[candidate] = true
			// If the dependency is a component, all transitive children of the dependency that are before this
			// resource in the topological sort are also implicitly dependencies. This is necessary because for remote
			// components, the dependencies will not include the transitive set of children directly, but will include
			// the parent component. We must walk that component's children here to ensure they are treated as
			// dependencies. Transitive children of the dependency that are after the resource in the topological sort
			// are not included as this could lead to cycles in the dependency order.
			if !candidate.Custom {
				for _, transitiveCandidateIndex := range dg.childrenOf[candidate.URN] {
					if transitiveCandidateIndex < cursorIndex {
						set[dg.resources[transitiveCandidateIndex]] = true
					}
				}
			}
		}
		// Include the resource's parent, as the resource depends on it's parent existing.
		if candidate.URN == res.Parent {
			set[candidate] = true
		}
	}

	return set
}

// NewDependencyGraph creates a new DependencyGraph from a list of resources.
// The resources should be in topological order with respect to their dependencies, including
// parents appearing before children.
func NewDependencyGraph(resources []*resource.State) *DependencyGraph {
	index := make(map[*resource.State]int)
	childrenOf := make(map[resource.URN][]int)

	urnIndex := make(map[resource.URN]int)
	for idx, res := range resources {
		index[res] = idx
		urnIndex[res.URN] = idx
		parent := res.Parent
		for parent != "" {
			childrenOf[parent] = append(childrenOf[parent], idx)
			parent = resources[urnIndex[parent]].Parent
		}
	}

	return &DependencyGraph{index, resources, childrenOf}
}
