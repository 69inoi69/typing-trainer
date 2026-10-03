# Architecture

## Overview

```
 Browser (web/)                         Go server
 ┌───────────────────────┐   JSON    ┌─────────────────────────────────────────┐
 │ app.js                │ ───────▶  │ api        HTTP handlers, /api/v1/*      │
 │  keydown, timing,     │ ◀───────  │   │                                      │
 │  keyboard, intro      │           │   ├──▶ engine   model + word generator   │
 └───────────────────────┘           │   └──▶ storage  Store interface          │
                                     │              └─ MemoryStore (now)        │
                                     │              └─ PostgresStore (later)    │
                                     │ config  all numbers   layout  key data   │
                                     └─────────────────────────────────────────┘
```

**Dependency rule:** `api` uses `engine` and `storage`. `storage` imports
`engine` only for the `KeyStat` type. `engine` imports nothing from the project.
The engine therefore never knows about HTTP or the database and can be tested on
its own.

## Packages and files

| Path | Responsibility |
|---|---|
| `main.go` | Loads the embedded data files, creates the store, generator and API server, starts HTTP. |
| `internal/config` | Every tunable number in one place. |
| `internal/engine/model.go` | Per-letter statistics, EMA update, difficulty, mastery, softmax weight. |
| `internal/engine/generator.go` | Unlock order, first lesson size, unlocking rule, word selection. |
| `internal/storage/store.go` | The `Store` interface. |
| `internal/storage/memory.go` | In-memory `Store` (lost on restart). |
| `internal/api/handlers.go` | JSON API, input validation, locking around read-modify-write. |
| `internal/layout` | Parses and validates the keyboard layout JSON. |
| `data/layouts/qwerty-en.json` | Keys, rows, finger per key, finger colors. |
| `data/words-en.txt` | Word list, most frequent first. |
| `web/` | Static frontend: `index.html`, `style.css`, `app.js`. |

The `web/` and `data/` folders are embedded into the binary (`//go:embed`), so
the program runs the same from any folder.

## Request lifecycle: from a keypress to the next exercise

1. `keydown` → `handleChar()` in `app.js` compares the key with the expected
   character. Wrong: mark red, count the error, cursor stays. Right: push a sample
   `{char, errors, ms}` and move on.
2. Last character typed → `POST /api/v1/results` with all samples, WPM and accuracy.
3. The handler (under a mutex) reads the stats from the `Store`, calls
   `engine.Apply`, saves them, asks `Generator.NextUnlocked` whether a letter
   opens, saves the unlock count, stores a session summary.
4. The browser calls `GET /api/v1/stats` (letter chips, heatmap, history) and
   `GET /api/v1/exercise`, which runs `Generator.Next` on the new statistics.

## API

All responses are JSON.

| Method | Path | Request | Response |
|---|---|---|---|
| GET | `/api/v1/layout` | | layout JSON |
| GET | `/api/v1/exercise` | | `{text, focus, unlocked: [..], locked: [..]}` |
| POST | `/api/v1/results` | `{samples: [{char, errors, ms}], wpm, accuracy}` | `{unlocked, newLetter}` |
| POST | `/api/v1/progress` | `{unlockAll: true\|false}` | `{unlocked}` |
| GET | `/api/v1/stats` | | `{keys: [{char, count, errorRate, avgMs, difficulty, mastered, unlocked}], sessions: [..], targetWpm}` |

Validation on `/results`: 1–5000 samples, each with exactly one character,
`errors >= 0`, `ms >= 0`; WPM 0–400; accuracy 0–100. Body limit 1 MB.

## Concurrency

The random generator is not safe for parallel use, so `Next` runs under
`genMu`. "Read stats → update → save" must not interleave between two
requests, so it runs under `writeMu`. A PostgreSQL store would use a
transaction (`SELECT ... FOR UPDATE`) instead.

## Adding PostgreSQL later

Create `internal/storage/postgres.go` with a type that has the six `Store`
methods and change one line in `main.go`. A possible schema, once accounts exist:

```sql
CREATE TABLE users     (id BIGSERIAL PRIMARY KEY, name TEXT UNIQUE NOT NULL,
                        unlocked INT NOT NULL DEFAULT 0, created_at TIMESTAMPTZ DEFAULT now());
CREATE TABLE key_stats (user_id BIGINT REFERENCES users(id), char TEXT,
                        count INT, mistakes INT, err_ema DOUBLE PRECISION,
                        time_ema DOUBLE PRECISION, timed_count INT,
                        PRIMARY KEY (user_id, char));
CREATE TABLE sessions  (id BIGSERIAL PRIMARY KEY, user_id BIGINT REFERENCES users(id),
                        wpm REAL, accuracy REAL, chars INT, finished_at TIMESTAMPTZ DEFAULT now());
```

With accounts, every `Store` method gets a `userID` parameter.

## Adding a layout or language

- New layout: a new JSON file in `data/layouts/` (keys, finger per key).
- New language: a new word list, most frequent first. The unlock order is
  computed from it automatically.
