c# AGENTS.md

## Project Overview

Shiori is a small, personal bookmark service written in Go.

The project is CLI-first. The initial implementation consists of a command-line interface backed by SQLite. A JavaScript-free web UI may be added later without changing the application/domain layer.

The primary goals are:

* Simple implementation
* Long-term maintainability
* Explicit SQL
* Minimal dependencies
* Easy operation by humans and AI agents
* Clear separation between domain/application logic and external interfaces

This is a personal-use application. Do not introduce multi-user architecture, distributed systems, or other infrastructure unless explicitly requested.

## Technology

The intended stack is:

* Go
* SQLite
* `database/sql`
* sqlc
* goose
* SQLite FTS5
* Go `html/template` for the future web UI
* Fedora-based Docker development environment
* fish shell
* mise for development tool versions
* Helix as the editor
* Starship for the shell prompt

Do not introduce an ORM.

Do not replace SQLite with another database unless explicitly requested.

Do not introduce a frontend framework.

## Architecture

The intended dependency direction is:

```text
CLI
 │
 ▼
Application Layer
 │
 ▼
sqlc-generated database code
 │
 ▼
database/sql
 │
 ▼
SQLite
```

The future web UI should use the same application layer:

```text
Browser
 │
 ▼
HTTP Handler
 │
 ▼
Application Layer
 │
 ▼
sqlc-generated database code
 │
 ▼
database/sql
 │
 ▼
SQLite
```

CLI and HTTP handlers are adapters. They must not contain business logic that belongs in the application layer.

The web UI must not access SQLite directly.

Keep the core logic as close as practical to a Functional Core / Imperative Shell design:

* Pure transformations and validation should be kept independent of I/O where practical.
* Database, filesystem, HTTP, and process interactions belong at the edges.
* Avoid unnecessary abstraction layers.

Do not create interfaces merely for mocking if they provide no meaningful architectural benefit.

## CLI

The executable name is:

```text
shiori
```

The CLI is the primary interface.

The initial command set is:

```text
shiori add <URL>
shiori list
shiori show <ID>
shiori search <QUERY>
shiori update <ID>
shiori delete <ID>
shiori tag add <ID> <TAG>
shiori tag remove <ID> <TAG>
```

The exact command syntax may evolve, but commands should remain predictable and composable.

Human-readable output is the default.

Machine-readable JSON output should be available where appropriate, for example:

```text
shiori search docker --json
```

JSON output is intended for scripts and AI agents. Keep its structure stable once established.

CLI conventions:

* Successful commands write normal output to stdout.
* Errors write diagnostics to stderr.
* Use meaningful non-zero exit codes on failure.
* Do not mix human-readable diagnostics into JSON stdout.
* Avoid interactive prompts unless explicitly required.
* Avoid TUI behavior in the core CLI.

The CLI should be usable without a terminal UI.

## Data Model

Bookmarks are the central entity.

A bookmark should support at least:

* URL
* User-provided comment
* Title when available
* Creation/update timestamps
* Flat tags

Tags are intentionally flat.

Do not introduce hierarchical tags, tag trees, or tag namespaces unless explicitly requested.

The initial database is SQLite.

SQLite FTS5 is used for full-text search where appropriate.

Tag filtering and full-text search should be composable.

Future functionality such as saving page bodies, embeddings, RAG, automatic tagging, or AI-generated summaries is out of scope for the initial implementation.

Design the schema so that such functionality can be added later without implementing it prematurely.

## SQL and Database Access

Use explicit SQL.

Do not introduce an ORM.

Use `database/sql` for database access and sqlc to generate typed Go code from SQL.

Responsibilities:

* goose: database schema migrations
* sqlc: typed Go code generation from SQL
* `database/sql`: runtime database access
* SQLite: persistent storage and FTS5

Do not mix migration logic with application queries.

Schema changes must be made through goose migrations.

Never modify an already-applied migration to change an existing database schema. Create a new migration instead.

SQL should remain readable and explicit.

Avoid hiding important queries behind generic repository abstractions.

## Migrations

Use goose for migrations.

Migration files belong under:

```text
db/migrations/
```

Migrations must be deterministic and safe to run repeatedly through the migration system.

SQLite locking/concurrency behavior should be considered when implementing application startup and migrations.

Do not assume multiple Shiori processes can safely perform schema initialization concurrently without testing it.

## Full-Text Search

Use SQLite FTS5 rather than introducing an external search service.

FTS5 synchronization should be handled consistently with the source bookmark data.

Search queries should be parameterized.

Do not construct SQL using string concatenation with user-provided search terms.

If SQLite FTS5 syntax requires special handling, isolate that handling in a small, explicit query implementation rather than introducing a large abstraction.

## Web UI

The web UI is a later phase.

When implemented, it must be:

* JavaScript-free
* Multi-Page Application (MPA)
* Server-side rendered
* Implemented with Go `html/template`
* Based on ordinary HTTP requests and HTML forms
* Based on POST/Redirect/GET where appropriate
* Simple to navigate
* Minimal in CSS and client-side complexity

Do not introduce:

* React
* Vue
* Svelte
* SPA architecture
* client-side application state
* complex JavaScript
* build pipelines for frontend code

HTML escaping should be handled by `html/template`.

The web UI should share the application layer with the CLI.

A JSON HTTP API is optional and should only be added when there is a concrete need.

## Error Handling

Errors should retain useful context.

Use Go error wrapping where appropriate:

```go
return fmt.Errorf("create bookmark: %w", err)
```

Do not silently ignore errors.

Do not panic for ordinary runtime errors.

Validate input at the boundary, but keep domain-level validation independent of CLI-specific parsing where practical.

## Testing

Testing is an important part of development. Write meaningful unit tests for application and domain logic, and do not treat tests as optional.

Prioritize tests for:

* Domain and application logic
* Input validation and error cases
* Bookmark operations
* Tag operations
* Search behavior
* SQL behavior that is non-trivial
* FTS5 search behavior
* Migration behavior
* CLI behavior where practical

Guidelines:

* New non-trivial behavior should normally include corresponding tests.
* When fixing a bug, add a regression test that reproduces the bug when practical.
* When modifying existing behavior, update the relevant tests as part of the same change.
* Prefer small, focused tests that verify observable behavior.
* Test both successful cases and important failure cases.
* Keep tests deterministic and independent of external services.
* Avoid tests that depend on timing, network access, or global machine state.
* Prefer table-driven tests when they make a set of related cases clearer.
* Do not write tests solely to increase code coverage.
* Do not sacrifice code clarity or introduce unnecessary abstractions solely to make testing easier.
* Keep pure domain logic easy to test without I/O.
* Database tests should use an isolated temporary SQLite database and should not modify the development database.
* Tests must not depend on the order in which other tests are executed.

Before considering a feature or bug fix complete:

1. Add or update the relevant tests.
2. Run the relevant tests.
3. Run the full test suite when practical.
4. Do not consider the implementation complete if the relevant tests fail.

Use Go's standard `testing` package unless another testing dependency provides a clear and necessary benefit.

Do not introduce mocks or interfaces merely for the sake of unit testing. Prefer testing real behavior and keeping I/O at the edges so that core logic can be tested directly.

Do not create large amounts of low-value test code merely to increase coverage.

## Security

Shiori is initially intended for personal use.

Do not add authentication, authorization, TLS termination, reverse proxies, or multi-user security architecture unless external access is explicitly introduced.

Nevertheless:

* Always parameterize SQL.
* Do not trust user-provided URLs, tags, comments, or search strings.
* Use `html/template` for HTML rendering.
* Do not execute bookmark URLs as code.
* Do not introduce shell execution for normal bookmark operations.
* Avoid unnecessary filesystem access.

If Shiori is later exposed beyond localhost, explicitly revisit authentication and network security before doing so.

## AI-Agent Compatibility

Shiori should be easy for AI agents to operate through the CLI.

Prefer:

```text
shiori search <query> --json
```

over requiring an interactive interface.

Commands should have:

* predictable arguments
* predictable exit status
* deterministic output where practical
* machine-readable JSON when requested
* useful stderr errors

Do not make an AI agent scrape human-oriented terminal formatting when a structured output mode can reasonably be provided.

Do not introduce an MCP server, RAG system, embeddings, or agent framework into the initial implementation merely because Shiori may eventually be used by AI.

## Project Structure

Prefer a small and conventional Go project structure.

A likely structure is:

```text
.
├── AGENTS.md
├── README.md
├── Dockerfile
├── go.mod
├── go.sum
├── opencode.json
├── cmd/
│   └── shiori/
├── internal/
│   ├── ...
│   └── db/
│       └── generated/
├── db/
│   ├── migrations/
│   └── queries/
└── data/
```

Do not create directories merely for architectural appearance.

Keep packages cohesive and small.

## Development Environment

The recommended development environment is a Fedora-based Docker container.

The development container provides:

* Fedora
* fish
* Helix
* OpenCode
* Go
* mise
* Starship
* SQLite
* standard development tools

The project directory is mounted into:

```text
/workspace/shiori
```

The host project directory is:

```text
~/workspace/shiori
```

Do not require the host machine to contain the development toolchain when the tool is available inside the container.

Do not mount the host home directory, SSH credentials, cloud credentials, or Docker socket into the development container unless there is an explicit requirement.

SQLite database files should live under the project directory, for example:

```text
data/bookmarks.db
```

This allows the database to persist through the host bind mount without requiring a separate Docker volume.

Database files should not be committed to Git.

## OpenCode Safety

OpenCode is expected to run inside the development container.

The container is the primary isolation boundary.

OpenCode permission rules are a secondary control and should not be treated as a complete security boundary.

Do not weaken the container isolation merely to make an AI agent more convenient to use.

In particular, do not mount:

```text
~/.ssh
~/.aws
~/.config
/var/run/docker.sock
```

or the entire host filesystem into the development container unless explicitly required.

Git is useful for recovering accidental source changes, but Git is not a security boundary.

## Dependencies

Prefer the Go standard library.

Add a dependency only when it provides clear value and avoids unnecessary implementation complexity.

The intended external dependencies are currently limited to infrastructure such as:

* sqlc
* goose
* SQLite driver

Do not add frameworks simply because they are popular.

Do not introduce an HTTP framework when the standard library is sufficient.

## Code Style

Follow standard Go conventions.

Prefer simple code over clever code.

Use small functions with clear responsibilities.

Avoid excessive abstraction.

Avoid premature generalization.

Do not optimize performance without evidence that it matters.

When two designs are both technically valid, prefer the one with:

1. fewer moving parts
2. fewer dependencies
3. simpler failure modes
4. easier testing
5. easier operation by a human or AI agent

## Scope Discipline

Do not implement future features unless explicitly requested.

Future ideas include:

* Web UI
* HTTP API
* authentication
* external VPS access
* page-body archiving
* automatic title/content extraction
* AI tagging
* embeddings
* RAG
* AI summaries

The existence of these ideas must not cause the initial implementation to become over-engineered.

When implementing a feature, modify the smallest reasonable part of the system and preserve the existing architecture.

