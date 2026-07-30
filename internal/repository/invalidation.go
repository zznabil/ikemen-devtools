package repository

import "sort"

// TransitiveDependents returns changed paths and every reverse dependency transitively.
func TransitiveDependents(changed []string, edges []DependencyEdge) []string {
	reverse := map[string][]string{}
	for _, e := range edges {
		reverse[e.TargetPath] = append(reverse[e.TargetPath], e.SourcePath)
	}
	seen := map[string]bool{}
	q := append([]string(nil), changed...)
	for len(q) > 0 {
		p := q[0]
		q = q[1:]
		if seen[p] {
			continue
		}
		seen[p] = true
		for _, d := range reverse[p] {
			if !seen[d] {
				q = append(q, d)
			}
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
