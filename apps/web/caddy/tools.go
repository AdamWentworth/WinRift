//go:build tools

// Package tools anchors build-time commands that are compiled into the
// production image but are not imported by the healthcheck helper.
package tools

import _ "github.com/caddyserver/caddy/v2/cmd/caddy"
