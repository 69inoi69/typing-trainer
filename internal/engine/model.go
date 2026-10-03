// Package engine contains the adaptive learning model.
//
// It knows NOTHING about HTTP or databases: statistics go in, numbers and
// exercise text come out. That keeps it easy to test and easy to explain.
//
// This file is the MATH: how one letter's statistics are updated and how
// they become a difficulty score. generator.go uses these scores to pick
// words and to decide when a new letter is unlocked.
// The full explanation with reasoning is in docs/ALGORITHM.md.
package engine

import "math"

// Params are the tunable numbers of the model. Values live in internal/config.
type Params struct {
	Alpha         float64 // EMA smoothing factor (0..1); bigger = reacts faster, forgets faster
	MaxIntervalMs float64 // a gap longer than this is a pause, not a reaction time

	TargetWPM    float64 // speed goal; a letter at this speed has speed difficulty 0
	SlowMs       float64 // per-letter time treated as "maximally slow" (speed difficulty 1)
	MaxErrorRate float64 // error rate treated as "maximally bad" (error difficulty 1)
	ErrorWeight  float64 // share of difficulty from mistakes (ErrorWeight + SpeedWeight = 1)
	SpeedWeight  float64 // share of difficulty from slowness

	PriorDifficulty float64 // what we assume about a letter before seeing it
	PriorStrength   float64 // how many samples the prior is "worth"

	Temperature float64 // softmax temperature: low = focus hard on weak letters, high = spread out

	MasteryMinSamples int     // a letter needs this many samples before it can count as mastered
	MasteryErrorRate  float64 // ...and a smoothed error rate at or below this

	StartLetters  int     // first lesson has at least this many letters
	MinWords      int     // ...and enough letters to build at least this many real words
	RepeatPenalty float64 // weight multiplier for a word already used in the same exercise
	ExerciseWords int     // words per exercise
}

// TargetMs converts TargetWPM to milliseconds per character.
// Standard definition: 1 word = 5 characters, so ms = 60000 / (WPM * 5).
func (p Params) TargetMs() float64 { return 12000 / p.TargetWPM }

// Sample is ONE occurrence of a character in an exercise, sent by the browser.
//   - Errors: wrong keys pressed before the right one (0 = clean)
//   - Ms:     time from the previous correct keypress; 0 = not measured
//     (first character of the exercise)
type Sample struct {
	Char   string  `json:"char"`
	Errors int     `json:"errors"`
	Ms     float64 `json:"ms"`
}

// KeyStat is everything we remember about one letter.
type KeyStat struct {
	Count      int     `json:"count"`      // occurrences typed (lifetime)
	Mistakes   int     `json:"mistakes"`   // occurrences with at least one error (lifetime)
	ErrEMA     float64 `json:"errEma"`     // smoothed probability of making a mistake
	TimeEMA    float64 `json:"timeEma"`    // smoothed time per keypress, ms (clean presses only)
	TimedCount int     `json:"timedCount"` // samples included in TimeEMA
}

// emaStep is the smoothing factor for the n-th sample.
//
// Plain EMA:   new = old + alpha * (x - old)
// Problem: the very first samples would be pulled towards the start value 0.
// Fix: for the first samples use 1/n (that is an ordinary average), and switch
// to alpha once 1/n gets smaller than alpha (after 1/alpha samples).
// So: a fair average while data is scarce, a "recent matters more" average later.
func emaStep(n int, alpha float64) float64 {
	return math.Max(alpha, 1/float64(n))
}

// Update adds one sample to a letter's statistics and returns the new stats.
func Update(s KeyStat, smp Sample, p Params) KeyStat {
	s.Count++
	miss := 0.0
	if smp.Errors > 0 {
		miss = 1
		s.Mistakes++
	}
	s.ErrEMA += emaStep(s.Count, p.Alpha) * (miss - s.ErrEMA)

	// Only clean presses tell us how fast the finger really is. A press after
	// a mistake includes "noticing the mistake" time, and long gaps are pauses.
	if smp.Errors == 0 && smp.Ms > 0 && smp.Ms <= p.MaxIntervalMs {
		s.TimedCount++
		s.TimeEMA += emaStep(s.TimedCount, p.Alpha) * (smp.Ms - s.TimeEMA)
	}
	return s
}

// Apply adds a whole exercise of samples. The input map is not modified.
func Apply(stats map[string]KeyStat, samples []Sample, p Params) map[string]KeyStat {
	out := make(map[string]KeyStat, len(stats))
	for k, v := range stats {
		out[k] = v
	}
	for _, smp := range samples {
		out[smp.Char] = Update(out[smp.Char], smp, p)
	}
	return out
}

func clamp01(x float64) float64 { return math.Max(0, math.Min(1, x)) }

// RawDifficulty is what the data alone says, from 0 (easy) to 1 (hard):
//
//	errorPart = clamp(ErrEMA / MaxErrorRate)
//	speedPart = clamp((TimeEMA - TargetMs) / (SlowMs - TargetMs))
//	raw       = ErrorWeight * errorPart + SpeedWeight * speedPart
//
// speedPart is 0 at (or faster than) the target speed and 1 at SlowMs.
// A letter with no clean timed press yet gets speedPart = 1 (we cannot
// call it fast if we never saw it typed cleanly).
func RawDifficulty(s KeyStat, p Params) float64 {
	errorPart := clamp01(s.ErrEMA / p.MaxErrorRate)
	speedPart := 1.0
	if s.TimedCount > 0 {
		speedPart = clamp01((s.TimeEMA - p.TargetMs()) / (p.SlowMs - p.TargetMs()))
	}
	return p.ErrorWeight*errorPart + p.SpeedWeight*speedPart
}

// Difficulty is the raw difficulty pulled towards a prior while data is scarce
// ("shrinkage", the same idea as a Bayesian average):
//
//	difficulty = (n * raw + k * prior) / (n + k)
//
// n = samples seen, k = PriorStrength. With n = 0 it equals the prior, with
// n much bigger than k it is almost the raw value. Without this, a letter typed
// correctly once would look "perfect" and would stop being practised.
func Difficulty(s KeyStat, p Params) float64 {
	n := float64(s.Count)
	k := p.PriorStrength
	if n+k == 0 {
		return p.PriorDifficulty
	}
	return (n*RawDifficulty(s, p) + k*p.PriorDifficulty) / (n + k)
}

// Mastered says whether a letter has reached the goal: enough samples,
// few mistakes, and at least target speed. This is a plain yes/no rule on
// the raw numbers so it is easy to explain to a user ("type e at 30 WPM
// with under 5% mistakes").
func Mastered(s KeyStat, p Params) bool {
	return s.Count >= p.MasteryMinSamples &&
		s.ErrEMA <= p.MasteryErrorRate &&
		s.TimedCount > 0 && s.TimeEMA <= p.TargetMs()
}

// SoftmaxWeight turns difficulty into a selection weight:
//
//	weight = exp(difficulty / Temperature)
//	P(letter) = weight / sum of all weights
//
// Temperature controls focus. With T = 0.25, a letter with difficulty 1
// is e^4 ~ 55 times more likely than one with difficulty 0. With a large T
// all letters become almost equally likely. exp() is never 0, so no letter
// ever disappears completely.
func SoftmaxWeight(difficulty float64, p Params) float64 {
	return math.Exp(difficulty / p.Temperature)
}
