// Package config keeps every tunable number in ONE place.
// Change values here, not inside the logic. What each number means and why
// it has this value is explained in docs/ALGORITHM.md ("Parameters").
package config

import "typingtrainer/internal/engine"

type Config struct {
	Addr      string // address the HTTP server listens on
	MaxEvents int    // safety limit for one results request
	Engine    engine.Params
}

func Default() Config {
	return Config{
		Addr:      "localhost:3000",
		MaxEvents: 5000,
		Engine: engine.Params{
			Alpha:         0.1,  // EMA: roughly the last ~20 occurrences of a letter matter
			MaxIntervalMs: 2000, // gaps over 2 s are pauses, ignored for speed

			TargetWPM:    30,   // goal speed = 400 ms per character
			SlowMs:       1000, // 1 s per character = as slow as it gets (speed difficulty 1)
			MaxErrorRate: 0.25, // a mistake on every 4th occurrence = as bad as it gets
			ErrorWeight:  0.6,  // accuracy matters a bit more than speed...
			SpeedWeight:  0.4,  // ...(the two weights must add up to 1)

			PriorDifficulty: 0.7, // before we see a letter, assume it is fairly hard
			PriorStrength:   5,   // the assumption is worth 5 real samples

			Temperature: 0.25, // difficulty 1 vs 0 -> about 55x more likely

			MasteryMinSamples: 20,   // at least 20 occurrences...
			MasteryErrorRate:  0.05, // ...under 5% mistakes, and at TargetWPM

			StartLetters:  6,   // first lesson: at least 6 letters
			MinWords:      30,  // ...and at least 30 real words to choose from
			RepeatPenalty: 0.3, // each earlier use in the exercise multiplies weight by 0.3
			ExerciseWords: 20,  // short exercises -> the model updates more often
		},
	}
}
