// Package store persists captured requests to PostgreSQL. Structured
// columns cover the fields queried and aggregated constantly (ts,
// source_ip, service, path, response_status, latency_ms); a JSONB column
// holds the variable-shaped parts (headers, body). Schema migrations live
// in /migrations.
package store
