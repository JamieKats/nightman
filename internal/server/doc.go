// Package server is the routing layer. It maps incoming requests (by port
// and path) to the registered honeypot.Service handlers, and wraps them
// with the cross-cutting middleware: per-IP rate limiting and full request
// capture.
//
// Routing is deliberately decoupled from the individual services so a
// handler can later be lifted out into its own container/process without a
// rewrite (see the "Future direction" note in docs/PROJECT_BRIEF.md).
package server
