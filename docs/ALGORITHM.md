# The adaptive algorithm

This document explains how the trainer decides what you type next, and why
each part is built the way it is. It is written as raw material for the
thesis: every formula here is exactly what the code does, and every number
comes from `internal/config/config.go`.

Code: `internal/engine/model.go` (the math) and `internal/engine/generator.go`
(unlocking and word selection). Tests that check each claim:
`internal/engine/*_test.go`.

---

## 1. The idea in one paragraph

The user types **real English words**. For every letter the program keeps a
short, constantly updated picture of how well the user types it: how often they
make a mistake on it and how long it takes. These two numbers are combined into
one **difficulty** between 0 and 1. Letters are introduced **gradually**, most
frequent first; a new letter opens only when every open letter is typed
accurately at the target speed. Inside the open letters, each next word is
chosen at random, but with probabilities shaped by difficulty, so weak letters
show up much more often and strong ones still appear sometimes. There is one
mode and no settings: the model alone decides what to practise.

## 2. Design decisions

| Decision | Alternative | Why this one |
|---|---|---|
| Real words only | Generated letter combinations (pseudo-words), as in keybr.com | Words are what people type in real life, they are easier to read and read faster, and they keep motivation up. Cost: early lessons have fewer possible texts. Handled by the unlock order and the `MinWords` rule (section 6). |
| Statistics per letter | Per key pair (bigram), per word | Simplest unit that can be explained and shown on the keyboard. Bigrams are a natural extension (section 10). |
| Smoothed recent statistics (EMA) | Lifetime totals | Skill changes. A lifetime error rate remembers mistakes from weeks ago forever. |
| Probabilistic choice (softmax) | Always train the single worst letter | Always drilling one letter is boring and over-fits; probabilities give focus *and* variety. |
| Gradual unlocking | All 26 letters from the start | A beginner cannot learn 26 finger positions at once. The same engine also serves experienced typists: they open everything in the intro. |
| Statistics, not a neural network | ML model | No training data is needed, every decision can be explained with a formula, and it runs instantly on the server. |

## 3. What is measured

For every **character occurrence** in the exercise the browser sends one sample:

```
{ char: "s", errors: 1, ms: 412 }
```

- `errors` – how many wrong keys were pressed before the right one. The cursor
  does not move on a wrong key, so each occurrence ends with the correct key.
- `ms` – time between the previous correct keypress and this one, measured with
  `keydown` and `performance.now()` (sub-millisecond resolution).

Two kinds of time are **ignored** for speed (but still count for accuracy):

1. the time of a press that came **after a mistake**, because it includes
   noticing and correcting the error, not finger speed;
2. gaps longer than `MaxIntervalMs = 2000 ms`, which are pauses.

The first character of an exercise has no previous press, so it gets `ms = 0`
(not measured).

Exercise-level numbers, shown to the user:

```
WPM      = (correct characters / 5) / minutes        (standard: 1 word = 5 characters)
accuracy = correct keypresses / all keypresses * 100
```

## 4. Per-letter statistics: exponential moving average

For each letter we keep two smoothed values:

- `ErrEMA` – the probability of making a mistake on this letter,
- `TimeEMA` – the typical time per keypress, from clean presses only.

Each new sample `x` updates the value:

```
value_new = value_old + a * (x - value_old)
a = max(Alpha, 1 / n)              n = number of samples so far, Alpha = 0.1
```

For errors `x = 1` if the occurrence had a mistake, otherwise `0`. For time
`x = ms`.

**Why `max(Alpha, 1/n)`.** A plain EMA starts at 0 and needs many samples
before it means anything. With `a = 1/n`, the first samples give an exact
ordinary average (1st sample: weight 1, 2nd: 1/2, ...). Once `1/n` drops below
`Alpha` (after 10 samples), the update becomes a normal EMA.

**What `Alpha = 0.1` means.** The weight of a sample is multiplied by
`(1 - Alpha) = 0.9` with each newer sample, so it halves every
`ln 0.5 / ln 0.9 ≈ 6.6` occurrences of the letter. In practice the model
"remembers" about the last 20 occurrences. A larger `Alpha` reacts faster but is
noisier.

Lifetime counters (`Count`, `Mistakes`) are kept too, but only for display and
for the "enough evidence" rules below.

## 5. From statistics to difficulty

### 5.1 Raw difficulty

```
TargetMs  = 60000 / (TargetWPM * 5) = 400 ms           (TargetWPM = 30)

errorPart = clamp( ErrEMA / MaxErrorRate )             MaxErrorRate = 0.25
speedPart = clamp( (TimeEMA - TargetMs) / (SlowMs - TargetMs) )     SlowMs = 1000 ms

raw = ErrorWeight * errorPart + SpeedWeight * speedPart      0.6 and 0.4
```

`clamp` limits a value to the range 0..1.

- `errorPart` is 0 with no mistakes and 1 when a quarter or more of the
  occurrences have a mistake. Error rates of real typists are small numbers,
  so dividing by 0.25 spreads them over the whole 0..1 range.
- `speedPart` is 0 at the target speed or faster and grows linearly to 1 at
  one second per character. Being faster than the target is not rewarded
  further: the goal is to reach the target on every letter.
- Mistakes weigh a bit more (0.6) than speed (0.4). Touch typing courses
  usually put accuracy first, because a fast but wrong movement is being
  memorised.
- A letter with no clean, timed press yet gets `speedPart = 1`.

### 5.2 Shrinkage towards a prior

A single correct press would make a letter look perfect. So the raw value is
pulled towards an assumed **prior** while there is little evidence:

```
difficulty = (n * raw + k * prior) / (n + k)        prior = 0.7, k = 5, n = samples
```

This is a Bayesian average (also called additive smoothing). With `n = 0` the
difficulty equals the prior, so a **new letter starts as fairly hard** and gets
practised. After 5 samples data and prior weigh the same; after 50 the data
weighs 10 times more.

### 5.3 Worked example

Seven letters are open. The user is good at `e` and struggles with `s`:

| Letter | n | ErrEMA | TimeEMA | errorPart | speedPart | raw | difficulty | weight `exp(d/T)` |
|---|---|---|---|---|---|---|---|---|
| e | 60 | 0.02 | 300 ms | 0.08 | 0 | 0.048 | 0.098 | 1.48 |
| s | 30 | 0.12 | 520 ms | 0.48 | 0.20 | 0.368 | 0.415 | 5.27 |

If the other five letters look like `e`, then `s` is chosen as the focus with
probability `5.27 / (5.27 + 6 * 1.48) ≈ 37 %`, instead of `1/7 ≈ 14 %` for
uniform choice.

## 6. Unlocking letters

**Order.** Letters are sorted by how often they occur in the word list. For the
included list this gives `e t a r o n s i l c h d u p m g w f b y v k x q j z`.
The order is computed from the data, not written by hand, so another language
only needs another word list.

**First lesson.** At least `StartLetters = 6` letters, and more if needed until
at least `MinWords = 30` real words can be built from them. With the included
list this gives 7 letters (`e t a r o n s`, 50 words). Without this rule the
first lesson would cycle through a handful of words.

**Mastery** of a letter is a plain yes/no rule on the smoothed values:

```
mastered = n >= 20  and  ErrEMA <= 0.05  and  TimeEMA <= TargetMs (400 ms)
```

In words: at least 20 occurrences, under 5 % mistakes, at 30 WPM or faster.
It is checked on the raw values (not on the shrunk difficulty) so it can be
explained to a user in one sentence.

**Next letter.** After each exercise: if every open letter is mastered, the
next letter in the order opens. At most one letter per exercise. A new letter
starts at the prior difficulty 0.7, which is higher than the difficulty of the
mastered letters, so it automatically becomes the focus.

**Experienced typists** choose "I can already touch type" in the intro and get
all letters at once. From then on the engine works only through focus.

## 7. Choosing the words

Every exercise has `ExerciseWords = 20` words. Only words made entirely of open
letters are allowed. Each word slot is filled in two steps.

**Step 1, what to train: pick a focus letter.**

```
P(letter) = exp(d_letter / T) / Σ exp(d_i / T)        T = Temperature = 0.25
```

This is the softmax (Boltzmann) distribution, used in the same way in
reinforcement learning for choosing between actions. The **temperature** `T` is
one knob for "how hard to push on weaknesses":

- difficulty 1 against difficulty 0 gives a ratio of `e^(1/0.25) = e^4 ≈ 55`;
- with `T → 0` it always picks the single worst letter;
- with a large `T` every letter is almost equally likely.

`exp` is never 0, so no letter disappears from practice.

**Step 2, how to train it: pick a word that contains the focus letter.**

```
D(word)   = average difficulty of the word's letters (each occurrence counts)
P(word)  ~ exp(D(word) / T) * RepeatPenalty^(times already used in this exercise)
```

Among words that contain the focus letter, the ones that also contain other weak
letters are preferred. `RepeatPenalty = 0.3` makes repeating a word inside one
exercise unlikely, and the same word never appears twice in a row.

**Focus shown to the user** is simply the open letter with the highest
difficulty.

**When does the model react?** Statistics are updated after each exercise, and
the next exercise is generated from the new values. Exercises are kept short (20
words, about 100 characters: 40 seconds at 30 WPM, 20 seconds at 60 WPM) so this
feels immediate. Updating after each word
would be possible, but the text would change under the user's eyes.

## 8. Parameters

All in `internal/config/config.go`.

| Name | Value | Meaning | Reason for the value |
|---|---|---|---|
| `Alpha` | 0.1 | EMA smoothing | half-life ≈ 6.6 occurrences, memory ≈ 20 |
| `MaxIntervalMs` | 2000 | longer gaps are pauses | a 2 s gap is far slower than any real keystroke |
| `TargetWPM` | 30 | goal speed (400 ms / char) | reachable goal for a beginner; raise it for advanced users |
| `SlowMs` | 1000 | "maximally slow" | 1 s per letter = hunting for the key |
| `MaxErrorRate` | 0.25 | "maximally inaccurate" | spreads realistic error rates over 0..1 |
| `ErrorWeight` / `SpeedWeight` | 0.6 / 0.4 | mix of the two parts | accuracy first, but speed matters |
| `PriorDifficulty` | 0.7 | assumption about an unseen letter | new letters must get practised |
| `PriorStrength` | 5 | how many samples the prior is worth | a few presses are not yet evidence |
| `Temperature` | 0.25 | focus strength | strong focus (×55) without excluding anything |
| `MasteryMinSamples` | 20 | evidence needed for mastery | ≈ one EMA memory length |
| `MasteryErrorRate` | 0.05 | max mistakes for mastery | 95 % accuracy, a common minimum goal |
| `StartLetters` / `MinWords` | 6 / 30 | first lesson | enough variety from the first minute |
| `RepeatPenalty` | 0.3 | less repetition inside one exercise | variety |
| `ExerciseWords` | 20 | words per exercise | short loop, model updates often |

The values were chosen by reasoning, not fitted to data. Tuning them on real
user data is a good subject for the evaluation chapter (section 11).

## 9. Known simplifications

- **Time belongs to a transition, not a letter.** The measured time is from the
  previous key to this one, so a slow `s` after `t` may really be a slow `t→s`
  movement. It is assigned to `s`.
- **One global user.** There are no accounts yet; all statistics belong to whoever
  uses the server.
- **Small word list.** About 1000 frequent words, collected by hand for the
  prototype. Rare letters (`q`, `j`, `x`, `z`) have few words.
- **One stubborn letter blocks progress.** If one letter never reaches mastery,
  nothing new opens. The focus pushes that letter, which is the intended
  behaviour, but a cap (for example "open anyway after N exercises") may be needed.
- **Letter frequency, not keyboard position,** decides the order. Courses often
  start from the home row instead. This could be added as a second sorting key.

## 10. Where your own model can go further

These are directions, not things that are built:

1. **Bigram statistics.** Keep the same `KeyStat` for pairs like `th`, `ng`, and
   add their difficulty to `D(word)`. Directly targets the transition problem
   from section 9.
2. **Word-level choice in one step.** Instead of "letter, then word", a single
   softmax over `D(word)` with a different aggregation (sum, max, mean).
3. **Forgetting over time.** Skill fades between sessions. Half-life regression
   (Settles & Meeder, 2016) models the probability of recall as a function of time
   since last practice; the same idea can raise the difficulty of letters not seen
   for days.
4. **Fitting parameters.** With logged data, choose `Alpha`, `T` and the weights
   that best predict the next mistake.

## 11. How to show that it works (ideas for the evaluation chapter)

- **Unit tests** already show the core properties: a weak letter appears at least
  twice as often (`TestNext_WeakLetterGetsFocus`), old mistakes fade
  (`TestUpdate_RecentDataWins`), one press is not evidence
  (`TestDifficulty_ShrinkageNeedsEvidence`), only open letters are used.
- **Simulation.** A simulated typist with a hidden error probability per letter
  that slowly improves when practised. Compare how fast the adaptive generator
  and a uniform random generator bring all letters below 5 % errors.
- **Small user study.** Two groups (adaptive vs. random words), the same total
  practice time, compare WPM and accuracy before and after.
