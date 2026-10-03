// Typing trainer frontend. Plain JavaScript, no frameworks.
//
// Flow:  load layout -> load exercise -> user types (keydown) -> every
// character becomes a "sample" {char, errors, ms} -> exercise done ->
// POST samples to /api/v1/results (server updates the model, maybe unlocks
// a letter) -> GET a new exercise built from the updated model.
"use strict";

const API = "/api/v1";

// ---------- page elements ----------
const $ = (id) => document.getElementById(id);
const el = {
  textBox: $("text-box"), text: $("text"), input: $("hidden-input"),
  keyboard: $("keyboard"), hint: $("finger-hint"),
  wpm: $("wpm"), acc: $("acc"),
  letters: $("letters"), lettersCaption: $("letters-caption"),
  result: $("result"), history: $("history"), toast: $("toast"),
  btnNew: $("btn-new"), btnIntro: $("btn-intro"), heat: $("heat-toggle"),
  intro: $("intro"),
};

// ---------- state ----------
let layout = null;         // keyboard layout from the server
const keyEls = {};         // char -> key <div>
const fingerOf = {};       // char -> finger id

let exercise = null;       // last /exercise answer: text, focus, unlocked, locked
let lastStats = null;      // last /stats answer
let text = "";
let spans = [];            // one <span> per character
let pos = 0;               // index of the character to type next
let wrongHere = 0;         // wrong presses at the current position
let samples = [];          // one {char, errors, ms} per character typed
let startTime = null;      // performance.now() of the first keypress
let lastTime = null;       // performance.now() of the previous correct keypress
let presses = 0, correctPresses = 0;
let busy = false;          // true while talking to the server between exercises
let popLetter = null;      // letter that was just unlocked (for the animation)

// ---------- server calls ----------
async function getJSON(path) {
  const res = await fetch(API + path);
  if (!res.ok) throw new Error(path + " -> " + res.status);
  return res.json();
}
async function postJSON(path, body) {
  const res = await fetch(API + path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(path + " -> " + res.status);
  return res.json();
}

// ---------- keyboard ----------
function makeKey(k) {
  const finger = layout.fingers[k.finger];
  const key = document.createElement("div");
  key.className = "key" + (k.home ? " home" : "");
  key.style.setProperty("--fc", finger.color);
  if (k.width) key.style.width = `calc(var(--u) * ${k.width} + var(--u) * .12 * ${k.width - 1})`;
  key.innerHTML = `<span class="heat"></span><span class="lbl"></span>`;
  key.querySelector(".lbl").textContent = k.label || k.char;
  key.title = finger.name;
  return key;
}

function drawKeyboard() {
  el.keyboard.innerHTML = "";
  layout.rows.forEach((row, i) => {
    const rowEl = document.createElement("div");
    rowEl.className = "kb-row";
    rowEl.style.paddingLeft = `calc(var(--u) * ${layout.offsets?.[i] ?? 0})`;
    for (const k of row) {
      const key = makeKey(k);
      rowEl.appendChild(key);
      keyEls[k.char] = key;
      fingerOf[k.char] = k.finger;
    }
    el.keyboard.appendChild(rowEl);
  });
  // the intro shows the home row (A S D F ... ; ) with the same colors
  const home = $("intro-home");
  home.innerHTML = "";
  for (const k of layout.rows[1].slice(0, 10)) home.appendChild(makeKey(k));
}

let nextKeyEl = null;
function highlightNextKey() {
  if (nextKeyEl) nextKeyEl.classList.remove("next");
  const ch = text[pos];
  nextKeyEl = keyEls[ch] || null;
  if (!nextKeyEl) { el.hint.innerHTML = "&nbsp;"; return; }
  nextKeyEl.classList.add("next");
  const finger = layout.fingers[fingerOf[ch]];
  el.hint.style.setProperty("--fc", finger.color);
  el.hint.innerHTML = `press <b>${ch === " " ? "space" : ch}</b> with your <b>${finger.name.toLowerCase()}</b>`;
}

function flashMiss(ch) {
  const k = keyEls[ch];
  if (!k) return;
  k.classList.remove("miss");
  void k.offsetWidth; // restart the CSS animation
  k.classList.add("miss");
}

// ---------- letters bar ----------
function renderLetters() {
  if (!exercise) return;
  const info = {};
  for (const k of lastStats?.keys ?? []) info[k.char] = k;
  const focusHasData = (info[exercise.focus]?.count ?? 0) > 0;

  const frag = document.createDocumentFragment();
  const add = (c, cls) => {
    const chip = document.createElement("span");
    chip.className = "chip " + cls;
    chip.textContent = c;
    if (c === popLetter) chip.classList.add("pop");
    frag.appendChild(chip);
  };
  for (const c of exercise.unlocked) {
    let cls = info[c]?.mastered ? "mastered" : "";
    if (c === exercise.focus && (focusHasData || c === popLetter)) cls += " focus";
    add(c, cls);
  }
  for (const c of exercise.locked) add(c, "locked");
  el.letters.replaceChildren(frag);

  // every key you do not need yet is dimmed on the keyboard
  const open = new Set([...exercise.unlocked, " "]);
  for (const [c, key] of Object.entries(keyEls)) key.classList.toggle("locked", !open.has(c));

  const openCount = exercise.unlocked.length;
  const all = openCount + exercise.locked.length;
  const mastered = exercise.unlocked.filter((c) => info[c]?.mastered).length;
  let caption;
  if (!lastStats?.sessions?.length) caption = `${openCount} of ${all} letters open. Type the text to begin.`;
  else if (focusHasData) caption = `focus: <b>${exercise.focus}</b> · mastered ${mastered} of ${openCount} open letters`;
  else caption = `mastered ${mastered} of ${openCount} open letters`;
  if (exercise.locked.length && lastStats?.sessions?.length)
    caption += ` · master all to open <b>${exercise.locked[0]}</b>`;
  el.lettersCaption.innerHTML = caption;
  popLetter = null;
}

// ---------- exercise ----------
function showExercise(ex) {
  exercise = ex;
  text = ex.text;
  pos = 0; wrongHere = 0; samples = [];
  startTime = null; lastTime = null; presses = 0; correctPresses = 0;

  // build all spans at once with a fragment: one DOM update, not hundreds
  const frag = document.createDocumentFragment();
  spans = [...text].map((ch) => {
    const s = document.createElement("span");
    s.textContent = ch;
    frag.appendChild(s);
    return s;
  });
  el.text.replaceChildren(frag);
  spans[0]?.classList.add("cur");
  el.textBox.classList.remove("typing");
  el.wpm.textContent = "0";
  el.acc.textContent = "100";
  highlightNextKey();
  renderLetters();
}

async function loadExercise() {
  showExercise(await getJSON("/exercise"));
}

// The core of the app, called once per typed character. Kept small on
// purpose: only the 1-2 spans that changed are touched, no layout reads.
function handleChar(ch) {
  if (busy || pos >= text.length || !el.intro.classList.contains("hidden")) return;
  const now = performance.now();
  if (startTime === null) startTime = now;
  el.textBox.classList.add("typing");
  el.result.classList.add("hidden");

  const expected = text[pos];
  presses++;

  if (ch === expected) {
    // ms = time since the previous correct press. Not measured (0) for the
    // first character; after a mistake the server ignores the time anyway.
    const ms = lastTime !== null ? now - lastTime : 0;
    samples.push({ char: expected, errors: wrongHere, ms: Math.round(ms) });
    correctPresses++;
    const span = spans[pos];
    span.classList.remove("cur", "bad");
    span.classList.add(wrongHere ? "fixed" : "ok");
    pos++; wrongHere = 0; lastTime = now;
    if (pos < text.length) spans[pos].classList.add("cur");
    highlightNextKey();
    if (pos === text.length) finishExercise();
  } else {
    wrongHere++;
    spans[pos].classList.add("bad");
    flashMiss(ch);
  }
  updateLive();
}

// WPM uses the standard definition: 5 characters = 1 word.
function currentWpm() {
  if (startTime === null) return 0;
  const minutes = (performance.now() - startTime) / 60000;
  return minutes > 0 ? (pos / 5) / minutes : 0;
}
function currentAccuracy() {
  return presses ? (correctPresses / presses) * 100 : 100;
}
function updateLive() {
  el.wpm.textContent = Math.round(currentWpm());
  el.acc.textContent = Math.round(currentAccuracy());
}
// keep WPM ticking while the user pauses
setInterval(() => { if (startTime !== null && pos < text.length) updateLive(); }, 500);

async function finishExercise() {
  busy = true;
  const wpm = Math.round(currentWpm() * 10) / 10;
  const accuracy = Math.round(currentAccuracy() * 10) / 10;
  el.result.innerHTML =
    `<div><b>${Math.round(wpm)}</b><small>wpm</small></div>` +
    `<div><b>${Math.round(accuracy)}%</b><small>accuracy</small></div>` +
    `<div><b>${text.length}</b><small>chars</small></div>`;
  el.result.classList.remove("hidden");
  try {
    const res = await postJSON("/results", { samples, wpm, accuracy });
    if (res.newLetter) {
      popLetter = res.newLetter;
      toast(`new letter: <b>${res.newLetter}</b> · find it on the keyboard before you start`);
    }
    await refreshStats();
    await loadExercise();   // generated from the UPDATED model
  } catch (err) {
    console.error(err);
    el.hint.textContent = "could not reach the server. Is it running?";
  } finally {
    busy = false;
  }
}

let toastTimer = null;
function toast(html) {
  el.toast.innerHTML = html;
  el.toast.classList.add("show");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.toast.classList.remove("show"), 4000);
}

// ---------- statistics: heatmap + history ----------
async function refreshStats() {
  lastStats = await getJSON("/stats");
  $("intro-target").textContent = lastStats.targetWpm;
  for (const k of lastStats.keys) {
    const key = keyEls[k.char];
    if (key) key.style.setProperty("--heat", k.count ? k.difficulty.toFixed(3) : 0);
  }
  const sessions = [...lastStats.sessions].reverse(); // oldest first
  if (!sessions.length) { el.history.innerHTML = ""; return; }
  const max = Math.max(...sessions.map((s) => s.wpm), 1);
  const bars = sessions.map((s) =>
    `<div class="bar" title="${Math.round(s.wpm)} wpm, ${Math.round(s.accuracy)}%" style="height:${(s.wpm / max) * 100}%"></div>`).join("");
  const weak = lastStats.keys.filter((k) => k.unlocked && k.count > 0 && !k.mastered).slice(0, 5)
    .map((k) => `<span>${k.char}</span>`).join("");
  el.history.innerHTML =
    `<h3>last ${sessions.length} exercises (wpm)</h3><div class="bars">${bars}</div>` +
    (weak ? `<div class="weak">weakest letters: ${weak}</div>` : "");
}

// ---------- intro ----------
let introStep = 0;
function showIntro() {
  introStep = 0;
  renderIntro();
  el.intro.classList.remove("hidden");
  $("intro-next").focus();
}
function renderIntro() {
  document.querySelectorAll(".intro-step").forEach((s, i) => s.classList.toggle("active", i === introStep));
  document.querySelectorAll("#intro-dots i").forEach((d, i) => d.classList.toggle("on", i === introStep));
  const last = introStep === 2;
  $("intro-next").style.visibility = last ? "hidden" : "visible";
  $("intro-skip").style.visibility = last ? "hidden" : "visible";
  if (last) $("intro-new").focus();
}
async function closeIntro(unlockAll) {
  el.intro.classList.add("hidden");
  if (unlockAll !== undefined) {
    try {
      const res = await postJSON("/progress", { unlockAll });
      // only swap the text if the set of letters really changed
      if (res.unlocked !== exercise?.unlocked.length) {
        await refreshStats();
        await loadExercise();
      }
    } catch (err) { console.error(err); }
  }
  focusText();
}
$("intro-next").addEventListener("click", () => { introStep++; renderIntro(); });
$("intro-skip").addEventListener("click", () => closeIntro());
$("intro-new").addEventListener("click", () => closeIntro(false));
$("intro-pro").addEventListener("click", () => closeIntro(true));
el.btnIntro.addEventListener("click", (e) => { e.currentTarget.blur(); showIntro(); });

// ---------- input ----------
// Desktop: keydown gives us the real key. preventDefault stops the character
// from ALSO arriving through the hidden input (and space from scrolling).
document.addEventListener("keydown", (e) => {
  if (!el.intro.classList.contains("hidden")) {
    if (e.key === "Escape") closeIntro();
    return; // let Enter/Tab work on the intro buttons
  }
  if (e.key === "Escape") { e.preventDefault(); newExercise(); return; }
  if (e.ctrlKey || e.metaKey || e.altKey) return;   // keep browser shortcuts
  if (e.key.length !== 1) return;                     // Shift, Tab, "Unidentified"...
  e.preventDefault();
  focusText();
  handleChar(e.key);
});

// Phones: virtual keyboards often report "Unidentified" on keydown, so the
// character only shows up here, in the hidden input. We read it and clear it.
el.input.addEventListener("input", () => {
  for (const ch of el.input.value) handleChar(ch);
  el.input.value = "";
});

const isTouch = () => matchMedia("(pointer: coarse)").matches;
function focusText() {
  if (isTouch() && document.activeElement !== el.input) el.input.focus({ preventScroll: true });
  el.textBox.classList.remove("blurred");
}
el.textBox.addEventListener("click", () => { el.input.focus({ preventScroll: true }); focusText(); });
el.input.addEventListener("blur", () => { if (isTouch()) el.textBox.classList.add("blurred"); });

function newExercise() {
  if (busy) return;
  el.result.classList.add("hidden");
  loadExercise().catch(console.error);
}
el.btnNew.addEventListener("click", (e) => { e.currentTarget.blur(); newExercise(); });
el.heat.addEventListener("change", () => {
  el.keyboard.classList.toggle("show-heat", el.heat.checked);
  el.heat.blur();
});

// ---------- start ----------
(async function init() {
  if (isTouch()) el.textBox.classList.add("blurred"); // phone: tap to open the keyboard
  try {
    layout = await getJSON("/layout");
    drawKeyboard();
    await refreshStats();
    await loadExercise();
    if (!lastStats.sessions.length) showIntro(); // first visit
  } catch (err) {
    console.error(err);
    el.text.textContent = "Could not load from the server. Is the Go server running?";
  }
})();
