[![GSoC 2026](https://img.shields.io/badge/GSoC-2026-red)](https://summerofcode.withgoogle.com)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)
[![Go Build](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![LLM: Groq](https://img.shields.io/badge/LLM-Groq-orange)](https://console.groq.com)
[![LLM: Gemini](https://img.shields.io/badge/LLM-Gemini-yellow)](https://aistudio.google.com)

# pgwatch AI Copilot

A natural language interface for [pgwatch](https://github.com/cybertec-postgresql/pgwatch) — ask questions about your PostgreSQL performance metrics in plain English, powered by AI.

## Overview

pgwatch AI Copilot lets database engineers analyze PostgreSQL performance metrics using natural language instead of writing SQL or navigating dashboards manually. It connects directly to the pgwatch metrics database, discovers the schema automatically, and uses an LLM to translate questions into accurate, scoped SQL queries — then explains the results in plain English.

Visit [pgwatch](https://github.com/cybertec-postgresql/pgwatch) for the full pgwatch documentation.

## Quick Start

### Prerequisites

- Go 1.21+
- pgwatch running with PostgreSQL metrics storage
- A free Groq API key → [console.groq.com](https://console.groq.com)

### Run
```shell
git clone https://github.com/Mohamed-Elfardy/pgwatch-copilot.git && cd pgwatch-copilot

go mod tidy

export PGWATCH_CONN="postgres://postgres@localhost:5432/pgwatch_metrics?sslmode=disable"
export GROQ_API_KEY="your-groq-api-key"
export PGWATCH_DBNAME="your-monitored-db-name"

go run .
```

Or using flags directly:
```shell
go run . --conn "postgres://..." --groq-key "..." --dbname "postgres-local"
```

## Example Session
```console
🔌 Connecting to pgwatch_metrics database...
✅ Connected!

🤖 pgwatch Copilot ready! (dbname: postgres-local, LLM: groq)
Type your question or 'exit' to quit.

You: How many active connections are there right now?

🔍 SQL:
SELECT data->>'numbackends' AS active_connections
FROM db_stats
WHERE dbname = 'postgres-local'
AND time > NOW() - INTERVAL '1 hour'
ORDER BY time DESC LIMIT 1;

📊 Results:
active_connections
──────────────────
2

💡 Insight:
There are currently 2 active connections to postgres-local.
This is a low number, suggesting the database is not under heavy load at the moment.

You: exit
👋 Bye!
```

## Architecture
```
User (Natural Language)
        ↓
   CLI Interface (cobra)
        ↓
  LLM Provider (Groq / Gemini)
  NL → SQL generation with schema context
        ↓
  pgwatch Metrics DB (PostgreSQL)
  scoped by dbname + JSONB field discovery
        ↓
  Results + AI Insight
```

## Project Structure
```
pgwatch-copilot/
├── main.go
├── cmd/
│   └── root.go              # CLI entry point
├── internal/
│   ├── llm/
│   │   ├── provider.go      # Common LLM interface
│   │   ├── gemini.go        # Google Gemini implementation
│   │   └── groq.go          # Groq (Llama) implementation
│   ├── metrics/
│   │   └── querier.go       # pgwatch DB layer with scoping
│   ├── profile/
│   │   └── cluster.go       # Persistent cluster metadata
│   └── copilot/
│       └── copilot.go       # Core engine
└── README.md
```

## Key Design Decisions

Based on the comments in slack and discussions with mentors **Ahmed Gouda** and **Pavlo Golub**:


**Pluggable LLM Providers**
The copilot is not tied to a specific model. A common `Provider` interface supports multiple backends — Groq, Gemini, and easily extensible to OpenAI, Claude, or local models via Ollama.
```go
type Provider interface {
    Complete(ctx context.Context, system string, messages []Message) (*Response, error)
    Name() string
}
```

**Multi-instance Awareness**
The copilot is aware that multiple pgwatch instances can write to the same storage. All queries are explicitly scoped by `dbname` and validated before execution to prevent data cross-contamination.

**JSONB Field Discovery**
pgwatch stores metrics as JSONB inside the `data` column. The copilot automatically samples the schema to discover actual field names (e.g. `numbackends`, `blks_hit`, `sessions`) and provides them to the LLM as context, ensuring accurate query generation.

**Cluster Profile**
A persistent `ClusterProfile` is stored per monitored database containing workload type (OLTP / OLAP / Mixed) and historical bottlenecks. This gives the copilot long-term context about each cluster without relying on outdated real-time data.

## Supported LLM Providers

| Provider | Free Tier | Model |
|----------|-----------|-------|
| Groq | ✅ Yes | llama-3.3-70b-versatile |
| Google Gemini | ✅ Yes | gemini-2.0-flash |

## Roadmap

- [ ] Multi-turn conversation with context memory
- [ ] Proactive optimization validation via plan change detection
- [ ] `sys_id` scoping for multi-instance pgwatch deployments
- [ ] Alert generation from anomaly detection
- [ ] HypoPG integration for hypothetical index analysis
- [ ] Integration with pgwatch REST API
- [ ] Streaming responses
- [ ] Export results to CSV / JSON

## Related Resources

- [pgwatch](https://github.com/cybertec-postgresql/pgwatch)
- [HypoPG — Hypothetical Indexes](https://github.com/HypoPG/hypopg)
- [Groq API](https://console.groq.com)

## Contributing

Feedback, suggestions, problem reports, and pull requests are very much appreciated.
