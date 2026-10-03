# Project rules for AI agents

## What this is

An adaptive touch-typing trainer: the practical part of my engineering thesis
(Lublin University of Technology, due 2027). It should look and feel like
Monkeytype, but teach: it tracks each letter's mistakes and speed and builds
exercises from real words around the user's weak letters.

## Who you are working with

- I am not an experienced programmer. Explain what you changed and why, in
  simple words, after every task. I must be able to defend every part of this
  code in front of a thesis committee.
- Talk to me in Russian. Code, comments, commit messages and docs: English.
- Work in small steps. For anything bigger than one feature, show a short plan
  first and wait for my OK.
- Do not run `git commit` or `git push` unless I ask. I commit myself.

## Stack (do not change without asking)

- Backend: Go, **standard library only** (`net/http`), no frameworks, no
  third-party modules.
- Frontend: plain HTML + CSS + vanilla JavaScript. **No frameworks, no npm,
  no build step.** Files in `web/` are served by the Go server.
- Storage: in memory now, behind the `storage.Store` interface. PostgreSQL
  comes later as a new implementation of the same interface.
- English only, QWERTY only, for now.
- Development machine: Windows, PowerShell.

## Architecture rules

- `internal/engine` is pure logic: it must not import anything about HTTP,
  JSON handlers or databases.
- All data access goes through `storage.Store`.
- Every tunable number lives in `internal/config/config.go`. No magic numbers
  in the logic.
- All API routes start with `/api/v1/`.
- Keyboard layouts are data (`data/layouts/*.json`), not code.
- Keep keypress handling in `web/app.js` light: no layout reads, no heavy work
  per keystroke.

Details: `docs/ARCHITECTURE.md`.

## Product decisions (do not change without asking)

- **Real words only.** No generated letter combinations like keybr.
- **One mode.** No difficulty switches; the algorithm decides what to practise.
- **The math in `internal/engine/model.go` is the core of my thesis.** Do not
  change formulas or parameter values on your own. If you think something
  should change, explain it and ask.
- If the algorithm's behaviour changes, update `docs/ALGORITHM.md` and the tests
  in the same task. The docs must always describe what the code really does.

Details: `docs/ALGORITHM.md`.

## Not now

User accounts, leaderboards, profile pages, other languages and layouts, themes,
frameworks. Structure code so they can be added later, but do not build them.

## Commands

```
go run .            # start the server (address in internal/config/config.go)
go test ./...       # run all tests
gofmt -w .          # format
go vet ./...        # static checks
```

## Before you say a task is done

1. `gofmt -w .`, `go vet ./...` and `go test ./...` all pass.
2. New engine logic has unit tests, including edge cases (no data yet, zero values).
3. Docs updated if behaviour or the API changed.
4. A short summary for me: what changed, which files, how to check it in the browser.
