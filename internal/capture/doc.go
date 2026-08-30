// Package capture records everything about an incoming probe: source IP
// and port, full headers (esp. Authorization, User-Agent, X-Api-Key), raw
// and parsed body, timestamp, response latency, which fake endpoint was
// hit, and connection duration. It hands finished records to the store.
package capture
