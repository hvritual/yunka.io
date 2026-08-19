package core

import "sort"

// Paths returns a stable snapshot of registered handler paths. The radix tree
// stays locked for the traversal so diagnostics cannot race concurrent route
// registration or deletion.
func (t *RouterHandleTree) Paths() []string {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	paths := make([]string, 0, t.size)
	recursiveWalk(t.root, func(path string, _ Handle) bool {
		paths = append(paths, path)
		return false
	})
	sort.Strings(paths)
	return paths
}
