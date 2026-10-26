package metrics

import "testing"

func TestParseGenerationSpeed(t *testing.T) {
	cases := []struct {
		name  string
		line  string
		want  float64
		found bool
	}{
		{
			name:  "llama.cpp print_timing line as the engine prints it",
			line:  "slot print_timing: id  0 | task 326 | n_gen =   9438, tg =   3.87 t/s, tg_3s =   5.29 t/s",
			want:  3.87,
			found: true,
		},
		{
			name:  "a whole-number rate",
			line:  "slot print_timing: id  0 | task 1 | tg = 42 t/s",
			want:  42,
			found: true,
		},
		{
			name:  "no spacing around the figure",
			line:  "tg=12.5 t/s",
			want:  12.5,
			found: true,
		},
		{
			name: "the average alone is not the current rate",
			line: "slot print_timing: id  0 | task 326 | tg_3s =   5.29 t/s",
		},
		{
			name: "a prompt-evaluation rate is not a generation rate",
			line: "slot print_timing: id  0 | pp = 610.50 t/s",
		},
		{
			name: "a zero rate is the absence it always was",
			line: "slot print_timing: id  0 | tg = 0.00 t/s",
		},
		{
			name: "the nonsense before the first token yields nothing",
			line: "slot print_timing: id  0 | tg = -nan t/s",
		},
		{
			name: "a rate label without the t/s unit is not the figure",
			line: "some other log line mentioning tg = 100",
		},
		{
			name: "an unrelated line carries no speed",
			line: "registering model 'Qwen3.6-27B'",
		},
		{
			name: "an identifier ending in tg does not match",
			line: "slot print_tg = 5 t/s",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ParseGenerationSpeed(c.line)
			if ok != c.found {
				t.Fatalf("found = %v, want %v for %q", ok, c.found, c.line)
			}
			if ok && got != c.want {
				t.Errorf("speed = %v, want %v", got, c.want)
			}
		})
	}
}

func TestParseGenerationSpeedPrefersInstantaneous(t *testing.T) {
	// Both figures on one line: the instantaneous tg, never the tg_3s average.
	line := "99.999 I slot print_timing: id  0 | task 9 | n_gen = 10, tg = 7.15 t/s, tg_3s = 99.99 t/s"
	got, ok := ParseGenerationSpeed(line)
	if !ok || got != 7.15 {
		t.Errorf("ParseGenerationSpeed = %v/%v, want 7.15/true", got, ok)
	}
}
