> Public snapshot (2026-11-09). The design notes are the reference and are not published.

# Diegesis docs

- [language](language.md) — the language's concept model, grammar principles, examples
- [file-format](file-format.md) — the `.dieg.md` file format: front matter, language blocks, prose
- [compiler](compiler.md) — compile pipeline, time model, knowledge derivation, CLI
- [checks](checks.md) — error/warning layers, lint rules, narrative patterns, waivers, `dieg test`
- [storage](storage.md) — PostgreSQL schema, migrations, reader model, metrics, AGE graph
- [rendering](rendering.md) — renderer, verification card, world-state digest, sentence patches, style backends

Start with the [quickstart](quickstart.md).

The single source of truth for the grammar is [`spec/grammar.ebnf`](../spec/grammar.ebnf). `sl` examples marked `<!-- check -->` are compiled by `TestDocsExamplesCompile` in `go test ./cmd/dieg`.
