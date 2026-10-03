# Typing Trainer — prototype

An adaptive touch-typing trainer (engineering thesis prototype).
Go backend (standard library only) + plain HTML/CSS/JS frontend.
The server watches which keys you miss or type slowly and puts them into the
next exercise more often.

## Run it on Windows (from zero)

1. **Install Go** (once). Go to <https://go.dev/dl/>, download the
   Windows installer (`go1.24.x.windows-amd64.msi` or newer, at least 1.22),
   run it and click Next until it finishes.
2. **Check it works.** Open a *new* PowerShell window
   (Start → type `powershell` → Enter) and run:
   ```
   go version
   ```
   You should see something like `go version go1.24.7 windows/amd64`.
3. **Go to the project folder** (where this README is), for example:
   ```
   cd C:\Users\YOUR_NAME\Downloads\typing-trainer
   ```
4. **Start the server:**
   ```
   go run .
   ```
   The first run takes a few seconds. Then you see
   `Typing trainer running. Open http://localhost:8080 in your browser`.
   Windows may ask about the firewall — "Allow" is fine (or Cancel; localhost works anyway).
5. **Open** <http://localhost:8080> in your browser and start typing.
6. **Stop** the server with `Ctrl + C` in PowerShell. All statistics are
   in memory, so they are reset when the server stops (PostgreSQL comes later).

Run the unit tests:
```
go test ./...
```

Optional: build one `.exe` you can double-click: `go build -o typing-trainer.exe .`

## How to use

- On the first visit a three-step intro explains the home position. At the end,
  choose "I'm new to this" (start with 7 letters) or "I can already touch type"
  (all letters open). You can reopen it with "how it works".
- Type the grey text. Wrong key = the letter turns red and you must press the
  right one before moving on.
- The row of letters at the top shows your progress: open letters, the letter in
  **focus** (yellow, your weakest right now), mastered letters (green dot), and
  locked letters (outlined). Master every open letter and the next one opens.
- The keyboard shows the next key in its finger color; keys you do not need yet
  are dimmed. `Esc` or "new exercise" gives a new text without saving.
- "difficulty heatmap" tints keys red by how hard they are for you.

## Documentation

- [`docs/ALGORITHM.md`](docs/ALGORITHM.md): the adaptive model, every formula
  and parameter, why it is built this way, known simplifications, ideas for
  evaluation. Start here for the thesis.
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md): packages, data flow, API,
  how to add PostgreSQL, layouts and languages.
- [`docs/LITERATURE.md`](docs/LITERATURE.md): reading list with sources.

## Project structure

```
typing-trainer/
├── main.go                       wires everything together, starts the server
├── data/
│   ├── words-en.txt              word list, most frequent first
│   └── layouts/qwerty-en.json    keyboard layout + finger mapping (data, not code)
├── docs/                         algorithm, architecture, reading list
├── internal/
│   ├── config/config.go          ALL tunable numbers
│   ├── engine/model.go           the math: EMA, difficulty, mastery, softmax
│   ├── engine/generator.go       unlocking letters, choosing words
│   ├── engine/*_test.go          unit tests
│   ├── storage/store.go          Store interface
│   ├── storage/memory.go         in-memory implementation
│   ├── layout/layout.go          loads and validates layout JSON
│   └── api/handlers.go           HTTP JSON API under /api/v1/
└── web/                          index.html, style.css, app.js
```

## The algorithm in short

```
per letter (EMA, a = max(0.1, 1/n)):   ErrEMA, TimeEMA
raw        = 0.6 * clamp(ErrEMA / 0.25) + 0.4 * clamp((TimeEMA - 400) / 600)
difficulty = (n * raw + 5 * 0.7) / (n + 5)
P(letter)  ~ exp(difficulty / 0.25)            -> focus letter
P(word)    ~ exp(mean difficulty of its letters / 0.25)   among words with that letter
mastered   = n >= 20, ErrEMA <= 5 %, TimeEMA <= 400 ms (30 WPM)
all open letters mastered -> next letter opens
```

Full explanation: [`docs/ALGORITHM.md`](docs/ALGORITHM.md).

## Replacing the word list

`data/words-en.txt` is a hand-made list of about 1000 frequent English words
(one or more per line, separated by spaces, **most frequent first**). For a
bigger, citable list, use SUBTLEX-US or `wordfreq` (see `docs/LITERATURE.md`).
With Python installed:

```
pip install wordfreq
python -c "from wordfreq import top_n_list; import re; open('data/words-en.txt', 'w').write(' '.join(w for w in top_n_list('en', 6000) if re.fullmatch('[a-z]+', w)))"
```

Then restart the server. The unlock order and first lesson are recomputed
automatically. wordfreq data is CC BY-SA 4.0: credit it if you publish.

## Adding PostgreSQL later

Write `internal/storage/postgres.go` with a type that has the same six
methods as `Store`, then change one line in `main.go`. A schema sketch is in
`docs/ARCHITECTURE.md`.
