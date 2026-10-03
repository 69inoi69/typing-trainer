package engine

import (
	"math/rand"
	"strings"
	"testing"
)

var testWords = strings.Fields(`the to and a in is it you that he was for on are as with
at be this have from or one had by not but what all were we when your can said there
use an each which she do how their if will up other about out many then them these so
some her would make like him into time has look two more write go see number no way
could people my than first water been call who oil its now find long down day did get
come made may part over new sound take only little work know place year live me back
give most very after thing our just name good sentence man think say great where help
through much before line right too mean old any same tell boy follow came want show
also around form three small set put end does another well large must big even such
because turn here why ask went men read need land different home us move try kind hand`)

func newTestGen(seed int64) *Generator {
	return NewGenerator(testWords, rand.New(rand.NewSource(seed)), testParams)
}

func TestWeightedPick(t *testing.T) {
	w := []float64{1, 3} // total 4: r in [0, .25) -> 0, [.25, 1) -> 1
	for _, c := range []struct {
		r    float64
		want int
	}{{0, 0}, {0.24, 0}, {0.25, 1}, {0.999, 1}} {
		if got := WeightedPick(w, c.r); got != c.want {
			t.Errorf("r=%v: got %d, want %d", c.r, got, c.want)
		}
	}
}

func TestWeightedPick_EdgeCases(t *testing.T) {
	if got := WeightedPick(nil, 0.5); got != -1 {
		t.Errorf("empty: got %d, want -1", got)
	}
	if got := WeightedPick([]float64{0, 0, 0, 0}, 0.6); got != 2 {
		t.Errorf("all zero (uniform fallback): got %d, want 2", got)
	}
	for _, r := range []float64{0, 0.3, 0.5, 0.999} {
		if WeightedPick([]float64{1, 0, 1}, r) == 1 {
			t.Errorf("r=%v picked a zero weight", r)
		}
	}
}

func TestLetterOrder_MostFrequentFirst(t *testing.T) {
	order := letterOrder([]string{"eee", "ee", "ta", "t"})
	if strings.Join(order, "") != "eta" {
		t.Fatalf("got %v, want e t a", order)
	}
}

func TestFirstLesson_HasEnoughRealWords(t *testing.T) {
	g := newTestGen(1)
	if g.Start() < testParams.StartLetters {
		t.Fatalf("start = %d, below StartLetters", g.Start())
	}
	if n := len(g.eligible(g.Start())); n < testParams.MinWords {
		t.Fatalf("first lesson has only %d words", n)
	}
}

func TestNext_UsesOnlyOpenLetters(t *testing.T) {
	g := newTestGen(2)
	ex := g.Next(nil, 0, testParams) // brand new user: no stats, unlocked = 0
	open := strings.Join(ex.Unlocked, "") + " "
	for _, r := range ex.Text {
		if !strings.ContainsRune(open, r) {
			t.Fatalf("letter %q is locked but appeared in %q", r, ex.Text)
		}
	}
	if n := len(strings.Fields(ex.Text)); n != testParams.ExerciseWords {
		t.Fatalf("got %d words, want %d", n, testParams.ExerciseWords)
	}
	if len(ex.Unlocked)+len(ex.Locked) != len(g.Order()) {
		t.Fatal("unlocked + locked must cover every letter")
	}
}

func TestNext_NoWordTwiceInARow(t *testing.T) {
	g := newTestGen(3)
	for i := 0; i < 50; i++ {
		w := strings.Fields(g.Next(nil, 0, testParams).Text)
		for j := 1; j < len(w); j++ {
			if w[j] == w[j-1] {
				t.Fatalf("repeated word %q in %v", w[j], w)
			}
		}
	}
}

// The core promise: a weak letter shows up clearly more often.
func TestNext_WeakLetterGetsFocus(t *testing.T) {
	all := len(newTestGen(0).Order())
	good := repeat(KeyStat{}, Sample{Ms: 250}, 60)
	bad := repeat(KeyStat{}, Sample{Errors: 1}, 20)
	bad = repeat(bad, Sample{Ms: 800}, 20)

	share := func(stats map[string]KeyStat, letter string) float64 {
		g := newTestGen(42)
		hit, total := 0, 0
		for i := 0; i < 200; i++ {
			for _, r := range g.Next(stats, all, testParams).Text {
				if r != ' ' {
					total++
					if string(r) == letter {
						hit++
					}
				}
			}
		}
		return float64(hit) / float64(total)
	}

	even := map[string]KeyStat{}
	for _, c := range newTestGen(0).Order() {
		even[c] = good
	}
	weak := map[string]KeyStat{}
	for k, v := range even {
		weak[k] = v
	}
	weak["w"] = bad

	before, after := share(even, "w"), share(weak, "w")
	if after < before*2 {
		t.Fatalf("'w' share went from %.3f to %.3f, expected at least 2x", before, after)
	}
	if ex := newTestGen(1).Next(weak, all, testParams); ex.Focus != "w" {
		t.Fatalf("focus = %q, want w", ex.Focus)
	}
}

func TestNextUnlocked(t *testing.T) {
	g := newTestGen(4)
	start := g.Start()
	stats := map[string]KeyStat{}
	if got := g.NextUnlocked(stats, 0, testParams); got != start {
		t.Fatalf("no stats: got %d, want %d (nothing mastered yet)", got, start)
	}
	mastered := repeat(KeyStat{}, Sample{Ms: 250}, 30)
	for _, c := range g.Order()[:start] {
		stats[c] = mastered
	}
	if got := g.NextUnlocked(stats, start, testParams); got != start+1 {
		t.Fatalf("all mastered: got %d, want %d", got, start+1)
	}
	// one weak letter blocks the next unlock
	stats[g.Order()[0]] = repeat(KeyStat{}, Sample{Errors: 1}, 30)
	if got := g.NextUnlocked(stats, start, testParams); got != start {
		t.Fatalf("one weak letter: got %d, want %d", got, start)
	}
	// never goes past the last letter
	if got := g.NextUnlocked(stats, 999, testParams); got != len(g.Order()) {
		t.Fatalf("clamp: got %d", got)
	}
}
