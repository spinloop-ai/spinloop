package metrics

import (
	"math"
	"regexp"
	"strconv"
)

// generationSpeed matches the instantaneous generation-rate figure in an
// engine's timing line, e.g. llama.cpp's
//
//	86.15.860.290 I slot print_timing: id  0 | task 326 | n_gen = 9438, tg =   3.87 t/s, tg_3s =   5.29 t/s
//
// The figure read is the current rate the engine names `tg`. The pattern is
// deliberately narrow: `tg\s*=` cannot match `tg_3s =` — the underscore that
// must follow `tg` there breaks the `\s*=` that must follow here — and the
// line's other rates (the three-second average, prompt-evaluation speeds)
// carry different labels, so none of them reads as the current rate.
var generationSpeed = regexp.MustCompile(`\btg\s*=\s*([0-9]+(?:\.[0-9]+)?)\s*t/s`)

// ParseGenerationSpeed extracts an engine's self-reported generation speed,
// in tokens per second, from one captured output line, reporting whether the
// line carried one. llama.cpp's print_timing lines carry it today; an engine
// that prints no such line simply never yields a speed — the absent stat, not
// a zero. A figure that is not a plausible positive rate (an unparsable
// number, a zero or negative rate) is no speed either: llama.cpp prints
// nonsense before a slot's first token, and "0 t/s" is indistinguishable
// from the absence that means "not generating".
func ParseGenerationSpeed(line string) (float64, bool) {
	m := generationSpeed.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0, false
	}
	return v, true
}
