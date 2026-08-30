// Package services defines the interface every mocked LLM service
// implements. Concrete implementations live in sub-packages (ollama,
// openai, vllm, anthropic).
package services

import "net/http"

// Service is one mocked LLM service (e.g. Ollama-compatible). Each Service
// owns its own routes and canned responses and knows nothing about the
// others, so it can be extracted into a standalone process later.
type Service interface {
	// Name is the short identifier recorded on every captured request
	// ("ollama", "openai", "vllm", "anthropic").
	Name() string

	// Routes returns the handler for this service. The routing layer
	// mounts it and layers rate limiting and request capture on top.
	Routes() http.Handler
}
