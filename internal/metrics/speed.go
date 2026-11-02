package metrics

import (
	"math"
	"regexp"
	"strconv"
)

// generationSpeed3s and generationSpeed match the generation-rate figures in
// an engine's timing line, e.g. llama.cpp's
//
//	slot print_timing: id  0 | task 326 | n_gen = 9438, tg =   3.87 t/s, tg_3s =   5.29 t/s
//
// The figure read is `tg_3s` where the line carries a usable one and `tg`
// otherwise: the three-second rate answers "how fast is it generating now",
// while `tg` averages the whole task and lags a rate that has changed. Older
// engines print only `tg`, and the fallback keeps them reporting. Both
// patterns are deliberately narrow: `\btg\s*=` cannot match `tg_3s =` — the
// underscore that must follow `tg` there breaks the `\s*=` that must follow
// here — and the line's other rates (prompt-evaluation speeds) carry
// different labels, so none of them reads as the generation rate.
var (
	generationSpeed3s = regexp.MustCompile(`\btg_3s\s*=\s*([0-9]+(?:\.[0-9]+)?)\s*t/s`)
	generationSpeed   = regexp.MustCompile(`\btg\s*=\s*([0-9]+(?:\.[0-9]+)?)\s*t/s`)
)

// ParseGenerationSpeed extracts an engine's self-reported generation speed,
// in tokens per second, from one captured output line, reporting whether the
// line carried one. llama.cpp's print_timing lines carry it today; an engine
// that prints no such line simply never yields a speed — the absent stat, not
// a zero. The three-second rate wins wherever it carries a usable figure, and
// the instantaneous figure answers wherever it does not, so a rate that has
// just changed reads now rather than after the task's average catches up. A
// figure that is not a plausible positive rate (an unparsable number, a zero
// or negative rate) is no speed either: llama.cpp prints nonsense before a
// slot's first token, and "0 t/s" is indistinguishable from the absence that
// means "not generating" — so an unusable three-second figure falls through
// to the instantaneous one rather than quenching the line.
func ParseGenerationSpeed(line string) (float64, bool) {
	for _, re := range []*regexp.Regexp{generationSpeed3s, generationSpeed} {
		if v, ok := firstRate(line, re); ok {
			return v, true
		}
	}
	return 0, false
}

// firstRate reads re's figure from the line, rejecting a value that is not a
// plausible positive rate.
func firstRate(line string, re *regexp.Regexp) (float64, bool) {
	m := re.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0, false
	}
	return v, true
}
