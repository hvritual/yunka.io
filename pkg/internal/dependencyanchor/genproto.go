//go:build yunka_dependency_anchor

// Package dependencyanchor contains build-tagged imports that make
// publish-time module graph compatibility constraints explicit. It is not
// part of Yunka's runtime graph.
package dependencyanchor

import _ "google.golang.org/genproto/googleapis/type/date"
