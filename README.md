# noticerender

Localized, multi-language template tool for email and SMS notifications.
It compiles a JSON template catalog into a standalone JSON template package,
then renders JSONL render requests in batch. Everything is offline and uses
only the Go standard library.

## Build

```sh
go build ./...
go test ./...
go build -o bin/noticerender ./cmd/noticerender
```

## Commands

```sh
# Compile a catalog into an immutable template package.
noticerender compile -catalog catalog.json -out package.json

# Render a batch of JSONL requests.
noticerender render -package package.json -requests requests.jsonl \
  -results results.jsonl -summary summary.json
```

Without `-results`, per-request results are written to stdout; without
`-summary`, the summary is written to stderr.

## Catalog format

```json
{
  "default_language": "en",
  "languages": {
    "en": {},
    "de": { "fallback": "en" },
    "de_AT": { "fallback": "de" },
    "fr": {}
  },
  "entries": [
    { "template": "welcome_email", "language": "en",
      "text": "Hi {name}, code {{CUR}} {code}" }
  ]
}
```

- `default_language` must be declared in `languages` and every template must
  have a text in it. The default language cannot declare a fallback.
- A non-default language may name at most one fallback language; when omitted
  it falls back to the default language.
- A reference to an unknown language or any fallback cycle fails compilation.
- The same `template`/`language` pair may not be registered twice.

## Message syntax

Messages are parsed from left to right:

- `{{` produces one literal `{`, `}}` one literal `}`.
- `{name}` is a placeholder. The name starts with an ASCII letter, followed
  only by letters, digits or underscores. It ends at the first `}`.
- A placeholder may be repeated. Every translation that exists for a template
  must use exactly the same placeholder set as the default-language text.
- Any other unpaired brace or illegal name character fails compilation.

Parameter values are inserted verbatim: braces inside parameter values are
never reinterpreted as template syntax.

## Requests and results

Each request is one JSON object per line:

```json
{"template":"welcome_email","language":"de_AT","params":{"name":"Ada","code":"42"}}
```

Resolution starts at the requested language and follows the fallback chain one
language at a time, stopping at the first language that has a text. Texts from
different languages are never concatenated.

Each result line reports:

- `line`: original request line number (blank lines are ignored).
- `used_language`: the language whose text was rendered.
- `path`: visited languages, always starting with the requested language and
  ending at `used_language` (e.g. `["de_AT","de"]`).
- `fallback`: true when `used_language` differs from the requested language.
- `output` on success, or `error` with a specific reason on failure.

Unknown templates and unknown target languages are explicit errors. Missing,
extra or non-string parameters are rejected (params must be a JSON object whose
values are all strings). One failed request never stops the batch.

The summary is:

```json
{ "total": 9, "success": 6, "failed": 3, "fallback_used": 2 }
```

An empty request file produces no result lines and an all-zero summary.

## Compilation guarantees

The entire catalog is validated before anything is written. Output goes to a
temporary file and is renamed into place only on success, so a failed compile
neither creates nor overwrites an existing package. The compiled package is
self-contained; later edits to the source catalog cannot affect packages
already rendered from it.

## Examples

See the `examples/` directory:

- `catalog.json` — multi-level fallback (`de_AT → de → en`), escapes.
- `requests.jsonl` — mixed successes, direct and multi-level fallbacks,
  parameter errors, unknown template/language, and braces inside params.
- `bad-catalog.json` — a placeholder-set conflict that must fail compilation.

```sh
go run ./cmd/noticerender compile -catalog examples/catalog.json -out examples/package.json
go run ./cmd/noticerender render  -package examples/package.json \
  -requests examples/requests.jsonl \
  -results examples/results.jsonl -summary examples/summary.json
```
