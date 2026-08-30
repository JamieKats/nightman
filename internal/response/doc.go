// Package response holds the canned template responses each mocked service
// serves. v1 is a small fixed pool (~10 per endpoint) selected at random
// per request — enough variety to avoid trivial byte-for-byte
// fingerprinting. No dynamic generation, no real inference.
package response
