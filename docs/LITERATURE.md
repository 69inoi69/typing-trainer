# Reading list

Grouped by the thesis chapter where each source is useful. Start with the ones
marked ★.

## How people type (theory chapter)

- ★ **Dhakal, V., Feit, A. M., Kristensson, P. O., Oulasvirta, A. (2018).**
  *Observations on Typing from 136 Million Keystrokes.* CHI 2018.
  Large online typing study: what separates fast typists from slow ones (rollover,
  error rates, inter-key intervals). Good source for typical WPM and error numbers.
  <https://userinterfaces.aalto.fi/136Mkeystrokes/>
- **Feit, A. M., Weir, D., Oulasvirta, A. (2016).** *How We Type: Movement
  Strategies and Performance in Everyday Typing.* CHI 2016.
  Motion-capture study: many fast typists do not use the ten-finger system;
  consistent finger-to-key mapping matters. Useful for discussing why the app
  shows which finger to use.
  <https://userinterfaces.aalto.fi/how-we-type/>
- **Salthouse, T. A. (1986).** *Perceptual, cognitive, and motoric aspects of
  transcription typing.* Psychological Bulletin, 99(3), 303–319.
  Classic review of typing skill; the source of many well-known typing
  phenomena (for example, that skilled typists look ahead in the text).
- **Logan, G. D., Crump, M. J. C. (2011).** *Hierarchical control of cognitive
  processes: The case for skilled typewriting.* Psychology of Learning and
  Motivation, 54. Words as the unit of planning, letters as the unit of
  execution. Supports the "real words" design decision.

## Learning and practice (motivation for adaptivity)

- ★ **Ericsson, K. A., Krampe, R. T., Tesch-Römer, C. (1993).** *The role of
  deliberate practice in the acquisition of expert performance.* Psychological
  Review, 100(3), 363–406. Practice that targets current weaknesses is what builds
  skill. The theoretical basis for "train where you stumble".
- **Settles, B., Meeder, B. (2016).** *A Trainable Spaced Repetition Model for
  Language Learning.* ACL 2016. Duolingo's half-life regression: a real, published
  adaptive learning model. A good example for the related-work section and for the
  "forgetting over time" extension.
  <https://aclanthology.org/P16-1174/>

## Measuring typing (methods chapter)

- ★ **Soukoreff, R. W., MacKenzie, I. S. (2003).** *Metrics for text entry
  research: An evaluation of MSD and KSPC, and a new unified error metric.*
  CHI 2003. Standard definitions of text-entry speed and error rates; cite it for
  the WPM and accuracy formulas.
  <https://www.yorku.ca/mack/chi03.html>

## The math used in the engine

- **Hunter, J. S. (1986).** *The exponentially weighted moving average.* Journal
  of Quality Technology, 18(4). A citable source for the EMA.
- **Sutton, R. S., Barto, A. G. (2018).** *Reinforcement Learning: An
  Introduction* (2nd ed.). MIT Press. Softmax (Boltzmann) action selection with a
  temperature parameter, chapter 2. Free online: <http://incompleteideas.net/book/the-book-2nd.html>
- **Gelman, A. et al. (2013).** *Bayesian Data Analysis* (3rd ed.). CRC Press.
  Shrinkage towards a prior (hierarchical models); background for section 5.2 of
  ALGORITHM.md. A short intuitive source is enough here, this book is for depth.

## Word lists (data)

- **Brysbaert, M., New, B. (2009).** *Moving beyond Kučera and Francis: A critical
  evaluation of current word frequency norms and the introduction of a new and
  improved word frequency measure for American English.* Behavior Research
  Methods, 41(4). The SUBTLEX-US frequency list, a scientifically grounded
  replacement for the hand-made list in `data/words-en.txt`.
- **wordfreq** (Robyn Speer), Python library with frequency lists for many
  languages: <https://github.com/rspeer/wordfreq>. Data license CC BY-SA 4.0,
  so credit it in the README if you use it.

## Existing products (for the market analysis, later)

- keybr.com, open source: <https://github.com/aradzie/keybr.com>
- Monkeytype, open source: <https://github.com/monkeytypegame/monkeytype>
