package toposort

import "fmt"

// Sort performs Kahn's topological sort on a DAG.
// adjacency[node] = list of nodes that must come BEFORE node (its dependencies).
// Returns nodes in execution order (dependencies first).
// Returns an error if a cycle is detected.
func Sort(adjacency map[string][]string) ([]string, error) {
	// Build in-degree map.
	inDegree := make(map[string]int, len(adjacency))

	for name := range adjacency {
		inDegree[name] = 0
	}

	for name, deps := range adjacency {
		for _, dep := range deps {
			// Only track edges between known nodes.
			if _, ok := adjacency[dep]; !ok {
				continue
			}
			inDegree[name]++
		}
	}

	// Collect nodes with zero in-degree as the starting set.
	queue := make([]string, 0, len(adjacency))
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	order := make([]string, 0, len(adjacency))
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		// For each dependency of this node, decrement its in-degree.
		for name, deps := range adjacency {
			for _, dep := range deps {
				if dep == node {
					inDegree[name]--
					if inDegree[name] == 0 {
						queue = append(queue, name)
					}
				}
			}
		}
	}

	if len(order) != len(adjacency) {
		// Identify the nodes involved in cycles.
		var cycle []string
		for name, deg := range inDegree {
			if deg > 0 {
				cycle = append(cycle, name)
			}
		}
		return nil, fmt.Errorf("cycle detected among: %v", cycle)
	}

	return order, nil
}
