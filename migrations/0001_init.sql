-- 0001_init.sql — initial capture schema.
-- See docs/PROJECT_BRIEF.md "Logging requirements" for the rationale
-- behind structured columns + a JSONB column for variable-shaped parts.

CREATE TABLE requests (
    id                     BIGGENERATED-PLACEHOLDER
);

-- NOTE: placeholder migration. Flesh out with the real DDL once the store
-- package lands. Rough intended shape:
--
-- CREATE TABLE requests (
--     id                     BIGSERIAL PRIMARY KEY,
--     ts                     TIMESTAMPTZ  NOT NULL DEFAULT now(),
--     source_ip              INET         NOT NULL,
--     source_port            INTEGER      NOT NULL,
--     service                TEXT         NOT NULL,  -- 'ollama' | 'openai' | 'vllm' | 'anthropic'
--     path                   TEXT         NOT NULL,
--     method                 TEXT         NOT NULL,
--     headers                JSONB        NOT NULL,
--     body                   JSONB,
--     ja3_fingerprint        TEXT,                   -- nullable; not populated in v1
--     response_status        SMALLINT     NOT NULL,
--     latency_ms             INTEGER      NOT NULL,
--     connection_duration_ms INTEGER
-- );
--
-- CREATE INDEX requests_ts_idx        ON requests (ts);
-- CREATE INDEX requests_source_ip_idx ON requests (source_ip);
-- CREATE INDEX requests_service_idx   ON requests (service);
-- -- Add a GIN index on headers/body if/when we need to search inside them.
-- -- Add requests_ja3_idx once ja3_fingerprint is actually populated.
