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
			name:  "llama.cpp print_timing line reads its three-second rate",
			line:  "slot print_timing: id  0 | task 326 | n_gen =   9438, tg =   3.87 t/s, tg_3s =   5.29 t/s",
			want:  5.29,
			found: true,
		},
		{
			name:  "an older engine's tg line falls back and reads",
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
			name:  "no spacing around the three-second figure",
			line:  "tg_3s=12.5 t/s",
			want:  12.5,
			found: true,
		},
		{
			name:  "the three-second rate alone yields the speed",
			line:  "slot print_timing: id  0 | task 326 | tg_3s =   5.29 t/s",
			want:  5.29,
			found: true,
		},
		{
			name:  "an unusable three-second figure falls back to tg",
			line:  "slot print_timing: id  0 | tg_3s =   0.00 t/s, tg =   6.20 t/s",
			want:  6.20,
			found: true,
		},
		{
			name:  "the nonsense before the first token falls back too",
			line:  "slot print_timing: id  0 | tg_3s = -nan t/s, tg =   6.20 t/s",
			want:  6.20,
			found: true,
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
			name: "a zero three-second rate is the absence too",
			line: "slot print_timing: id  0 | tg_3s = 0.00 t/s",
		},
		{
			name: "a rate label without the t/s unit is not the figure",
			line: "some other log line mentioning tg = 100",
		},
		{
			name: "a three-second label without the t/s unit is not the figure",
			line: "some other log line mentioning tg_3s = 100",
		},
		{
			name: "an unrelated line carries no speed",
			line: "registering model 'Qwen3.6-27B'",
		},
		{
			name: "an identifier ending in tg does not match",
			line: "slot print_tg = 5 t/s",
		},
		{
			name: "an identifier ending in tg_3s does not match",
			line: "slot print_tg_3s = 5 t/s",
		},
		{
			name: "a longer average label does not masquerade as tg_3s",
			line: "slot print_timing: id  0 | tg_30s = 5.00 t/s",
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

func TestParseGenerationSpeedPrefersThreeSecondRate(t *testing.T) {
	// Both figures on one line: the three-second tg_3s, never the whole-task tg average.
	line := "99.999 I slot print_timing: id  0 | task 9 | n_gen = 10, tg = 7.15 t/s, tg_3s = 4.02 t/s"
	got, ok := ParseGenerationSpeed(line)
	if !ok || got != 4.02 {
		t.Errorf("ParseGenerationSpeed = %v/%v, want 4.02/true", got, ok)
	}
}
