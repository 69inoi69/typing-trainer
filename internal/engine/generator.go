package engine

import (
	"math/rand"
	"sort"
	"strings"
)

// Generator builds exercises out of REAL words (no random letter soup).
//
// Two ideas work together:
//
//  1. Unlocking. Letters are opened one at a time, most frequent first.
//     Only words made entirely of open letters can appear. A new letter
//     opens when every open letter is mastered (see Mastered in model.go).
//
//  2. Focus. Inside the open letters, weak letters are chosen more often
//     (softmax over difficulty), and words that contain them are preferred.
type Generator struct {
	words []string // the dictionary, most frequent first
	order []string // unlock order: letters sorted by frequency in the dictionary
	start int      // how many letters the first lesson has
	rng   *rand.Rand
}

// NewGenerator computes the unlock order and the first lesson size once.
func NewGenerator(words []string, rng *rand.Rand, p Params) *Generator {
	g := &Generator{words: words, rng: rng}
	g.order = letterOrder(words)
	g.start = g.firstLessonSize(p)
	return g
}

// letterOrder sorts letters by how often they appear in the word list.
// For English this gives roughly e t a r o n s i l c ... so the first lessons
// already allow many real words.
func letterOrder(words []string) []string {
	count := map[string]int{}
	for _, w := range words {
		for _, r := range w {
			count[string(r)]++
		}
	}
	order := make([]string, 0, len(count))
	for c := range count {
		order = append(order, c)
	}
	sort.Slice(order, func(i, j int) bool {
		if count[order[i]] != count[order[j]] {
			return count[order[i]] > count[order[j]]
		}
		return order[i] < order[j] // tie-break: alphabetical, so the order is stable
	})
	return order
}

// firstLessonSize: start with StartLetters, but add letters until at least
// MinWords real words can be built. Otherwise the first lesson would repeat
// the same 5 words forever.
func (g *Generator) firstLessonSize(p Params) int {
	n := p.StartLetters
	if n > len(g.order) {
		n = len(g.order)
	}
	for n < len(g.order) && len(g.eligible(n)) < p.MinWords {
		n++
	}
	return n
}

// Order returns the unlock order. Start returns the first lesson size.
func (g *Generator) Order() []string { return g.order }
func (g *Generator) Start() int      { return g.start }

// Clamp keeps an unlocked count inside [Start, number of letters].
// A stored value of 0 means "new user" and becomes Start.
func (g *Generator) Clamp(unlocked int) int {
	if unlocked < g.start {
		return g.start
	}
	if unlocked > len(g.order) {
		return len(g.order)
	}
	return unlocked
}

// NextUnlocked decides whether to open the next letter: yes if every open
// letter is mastered. At most one letter per call (per exercise), so the
// user is never flooded with new keys.
func (g *Generator) NextUnlocked(stats map[string]KeyStat, unlocked int, p Params) int {
	unlocked = g.Clamp(unlocked)
	if unlocked == len(g.order) {
		return unlocked
	}
	for _, c := range g.order[:unlocked] {
		if !Mastered(stats[c], p) {
			return unlocked
		}
	}
	return unlocked + 1
}

// eligible returns the words that use only the first n letters of the order.
func (g *Generator) eligible(n int) []string {
	open := map[rune]bool{}
	for _, c := range g.order[:n] {
		open[rune(c[0])] = true
	}
	var out []string
	for _, w := range g.words {
		ok := true
		for _, r := range w {
			if !open[r] {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, w)
		}
	}
	return out
}

// Exercise is what the generator returns.
type Exercise struct {
	Text     string   `json:"text"`
	Focus    string   `json:"focus"`    // the currently weakest open letter
	Unlocked []string `json:"unlocked"` // open letters, in unlock order
	Locked   []string `json:"locked"`   // letters still to come, in order
}

// Next builds one exercise.
//
// For every word slot:
//  1. pick a FOCUS letter: P(letter) ~ exp(difficulty / T)        (what to train)
//  2. take the open words that contain it, and pick one with
//     P(word) ~ exp(meanDifficulty(word) / T) * RepeatPenalty^uses (how to train it)
//
// Step 2 prefers words with several weak letters (e.g. both 'r' and 's'
// when both are weak) and avoids repeating a word within one exercise.
func (g *Generator) Next(stats map[string]KeyStat, unlocked int, p Params) Exercise {
	unlocked = g.Clamp(unlocked)
	open := g.order[:unlocked]
	words := g.eligible(unlocked)

	ex := Exercise{Unlocked: append([]string{}, open...), Locked: append([]string{}, g.order[unlocked:]...)}
	if len(words) == 0 {
		return ex
	}

	// difficulty and softmax weight of every open letter
	diff := map[rune]float64{}
	letterW := make([]float64, len(open))
	best := -1.0
	for i, c := range open {
		d := Difficulty(stats[c], p)
		diff[rune(c[0])] = d
		letterW[i] = SoftmaxWeight(d, p)
		if d > best {
			best, ex.Focus = d, c
		}
	}

	// group words by letter, and pre-compute each word's base weight
	byLetter := map[string][]int{}
	baseW := make([]float64, len(words))
	for i, w := range words {
		sum := 0.0
		seen := map[rune]bool{}
		for _, r := range w {
			sum += diff[r]
			if !seen[r] {
				seen[r] = true
				byLetter[string(r)] = append(byLetter[string(r)], i)
			}
		}
		baseW[i] = SoftmaxWeight(sum/float64(len(w)), p)
	}

	uses := make([]int, len(words))
	out := make([]string, 0, p.ExerciseWords)
	last := -1
	for len(out) < p.ExerciseWords {
		focus := open[WeightedPick(letterW, g.rng.Float64())]
		cands := byLetter[focus]
		if len(cands) == 0 { // a letter with no open word: pick from all words
			cands = make([]int, len(words))
			for i := range words {
				cands[i] = i
			}
		}
		w := make([]float64, len(cands))
		for j, idx := range cands {
			w[j] = baseW[idx]
			for u := 0; u < uses[idx]; u++ {
				w[j] *= p.RepeatPenalty
			}
			if idx == last && len(words) > 1 {
				w[j] = 0 // never the same word twice in a row
			}
		}
		pick := WeightedPick(w, g.rng.Float64())
		idx := cands[pick]
		if idx == last && len(words) > 1 { // only possible if every weight was 0
			continue
		}
		uses[idx]++
		last = idx
		out = append(out, words[idx])
	}
	ex.Text = strings.Join(out, " ")
	return ex
}

// WeightedPick picks an index with probability proportional to its weight.
// r must be a random number in [0, 1). Taking r as a parameter keeps the
// function deterministic, which is what the unit tests need.
//
// Picture the weights as segments laid end to end; r * total is a point on
// that line and we return the segment it lands in. All-zero weights fall
// back to a uniform pick. Empty input returns -1.
func WeightedPick(weights []float64, r float64) int {
	if len(weights) == 0 {
		return -1
	}
	total := 0.0
	for _, w := range weights {
		if w > 0 {
			total += w
		}
	}
	if total == 0 {
		return int(r * float64(len(weights)))
	}
	point := r * total
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		if point < w {
			return i
		}
		point -= w
	}
	// float rounding: return the last index with a positive weight
	for i := len(weights) - 1; i >= 0; i-- {
		if weights[i] > 0 {
			return i
		}
	}
	return len(weights) - 1
}
