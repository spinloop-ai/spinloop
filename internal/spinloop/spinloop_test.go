package spinloop

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Selection
	}{
		{
			name: "provider and model",
			in:   "PROVIDER openrouter\nMODEL deepseek/deepseek-v4-pro\n",
			want: Selection{Provider: "openrouter", Model: "deepseek/deepseek-v4-pro"},
		},
		{
			name: "case-insensitive keywords",
			in:   "provider ollama\nModel llama3.2\n",
			want: Selection{Provider: "ollama", Model: "llama3.2"},
		},
		{
			name: "model only",
			in:   "PROVIDER llamacpp\nMODEL gemma-4-12b-it\n",
			want: Selection{Provider: "llamacpp", Model: "gemma-4-12b-it"},
		},
		{
			name: "comments, blanks, and inline comments",
			in:   "# my Spinloop\n\nPROVIDER openrouter   # the provider\nMODEL  m1\t# inline tab comment\n",
			want: Selection{Provider: "openrouter", Model: "m1"},
		},
		{
			name: "extra whitespace and tabs as separator",
			in:   "PROVIDER\tollama\nMODEL     llama3.2\n",
			want: Selection{Provider: "ollama", Model: "llama3.2"},
		},
		{
			name: "context, output, and base url",
			in:   "PROVIDER llamacpp\nMODEL gemma\nCONTEXT 128k\nOUTPUT 32k\nBASEURL http://localhost:9090/v1\n",
			want: Selection{Provider: "llamacpp", Model: "gemma", Context: "128k", Output: "32k", BaseURL: "http://localhost:9090/v1"},
		},
		{
			name: "parallel",
			in:   "PROVIDER llamacpp\nMODEL gemma\nCONTEXT 128k\nPARALLEL 2\n",
			want: Selection{Provider: "llamacpp", Model: "gemma", Context: "128k", Parallel: "2"},
		},
		{
			name: "base url aliases",
			in:   "PROVIDER openai-compatible\nMODEL m\nURL https://gw/v1\n",
			want: Selection{Provider: "openai-compatible", Model: "m", BaseURL: "https://gw/v1"},
		},
		{
			name: "alias and preset",
			in:   "PROVIDER llamacpp\nMODEL unsloth/Qwen:Q4_K_M\nALIAS qwen\nPRESET ./preset.ini\n",
			want: Selection{Provider: "llamacpp", Model: "unsloth/Qwen:Q4_K_M", Alias: "qwen", Preset: "./preset.ini"},
		},
		{
			name: "repeated env",
			in:   "PROVIDER llamacpp\nMODEL m\nENV AWS_PROFILE=dev\nENV AWS_REGION=eu-west-2\n",
			want: Selection{Provider: "llamacpp", Model: "m", Env: []EnvVar{{Key: "AWS_PROFILE", Value: "dev"}, {Key: "AWS_REGION", Value: "eu-west-2"}}},
		},
		{
			name: "env with empty value",
			in:   "PROVIDER llamacpp\nMODEL m\nENV AWS_PROFILE=\n",
			want: Selection{Provider: "llamacpp", Model: "m", Env: []EnvVar{{Key: "AWS_PROFILE", Value: ""}}},
		},
		{
			name: "env value keeps later equals signs",
			in:   "PROVIDER llamacpp\nMODEL m\nENV TOKEN=a=b=c\n",
			want: Selection{Provider: "llamacpp", Model: "m", Env: []EnvVar{{Key: "TOKEN", Value: "a=b=c"}}},
		},
		{
			name: "a param",
			in:   "PROVIDER llamacpp\nMODEL m\nPARAM jinja = 1\n",
			want: Selection{Provider: "llamacpp", Model: "m", Params: []Param{{Key: "jinja", Value: "1"}}},
		},
		{
			name: "repeated params with a trailing comment",
			in: "PROVIDER llamacpp\nPARAM jinja = 1\nPARAM temp = 1.0\n" +
				"PARAM hf = unsloth/Qwen3.8-27B-GGUF:UD-Q5_K_XL   # repo and quant\n" +
				"PARAM no-mmproj = 1\n",
			want: Selection{
				Provider: "llamacpp",
				Params: []Param{
					{Key: "jinja", Value: "1"},
					{Key: "temp", Value: "1.0"},
					{Key: "hf", Value: "unsloth/Qwen3.8-27B-GGUF:UD-Q5_K_XL"},
					{Key: "no-mmproj", Value: "1"},
				},
			},
		},
		{
			name: "repeated param keys keep file order",
			in:   "PROVIDER llamacpp\nMODEL m\nPARAM temp = 0.5\nPARAM temp = 1.0\n",
			want: Selection{Provider: "llamacpp", Model: "m", Params: []Param{{Key: "temp", Value: "0.5"}, {Key: "temp", Value: "1.0"}}},
		},
		{
			name: "param with an empty value",
			in:   "PROVIDER llamacpp\nMODEL m\nPARAM no-mmap =\n",
			want: Selection{Provider: "llamacpp", Model: "m", Params: []Param{{Key: "no-mmap", Value: ""}}},
		},
		{
			name: "bare param names a flag with no value",
			in:   "PROVIDER llamacpp\nMODEL m\nPARAM jinja\n",
			want: Selection{Provider: "llamacpp", Model: "m", Params: []Param{{Key: "jinja", Value: ""}}},
		},
		{
			name: "param value keeps spaces and later equals signs",
			in:   "PROVIDER llamacpp\nMODEL m\nPARAM stop = a b = c\n",
			want: Selection{Provider: "llamacpp", Model: "m", Params: []Param{{Key: "stop", Value: "a b = c"}}},
		},
		{
			name: "param alongside preset and env",
			in:   "PROVIDER llamacpp\nMODEL m\nPRESET ./preset.ini\nPARAM ngl = 99\nENV AWS_PROFILE=dev\n",
			want: Selection{
				Provider: "llamacpp", Model: "m", Preset: "./preset.ini",
				Params: []Param{{Key: "ngl", Value: "99"}},
				Env:    []EnvVar{{Key: "AWS_PROFILE", Value: "dev"}},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse([]byte(tc.in))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParse_Errors(t *testing.T) {
	cases := map[string]string{
		"missing provider":           "MODEL llama3.2\n",
		"unknown keyword":            "PROVIDER ollama\nFLAVOUR vanilla\n",
		"family retired":             "PROVIDER openrouter\nFAMILY deepseek-v4\n",
		"keyword no value":           "PROVIDER\n",
		"too many values":            "PROVIDER a b\n",
		"duplicate keyword":          "PROVIDER a\nPROVIDER b\n",
		"duplicate parallel":         "PROVIDER a\nMODEL m\nPARALLEL 1\nPARALLEL 2\n",
		"duplicate alias":            "PROVIDER a\nMODEL m\nBASEURL u1\nURL u2\n",
		"env without equals":         "PROVIDER a\nMODEL m\nENV JUST_A_NAME\n",
		"env empty key":              "PROVIDER a\nMODEL m\nENV =value\n",
		"env value with spaces":      "PROVIDER a\nMODEL m\nENV KEY=hello world\n",
		"param empty key":            "PROVIDER a\nMODEL m\nPARAM =1\n",
		"param key with spaces":      "PROVIDER a\nMODEL m\nPARAM hf repo = m\n",
		"bare param key with spaces": "PROVIDER a\nMODEL m\nPARAM hf repo\n",
		"bare param with no key":     "PROVIDER a\nMODEL m\nPARAM \n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(in)); err == nil {
				t.Errorf("expected error for %q", in)
			}
		})
	}
}

func TestFormatRoundTrip(t *testing.T) {
	sel := Selection{
		Provider: "openrouter",
		Model:    "deepseek/deepseek-v4-pro",
		Alias:    "deepseek",
		Context:  "128000",
		Output:   "32000",
		Parallel: "2",
		BaseURL:  "https://gateway.example/v1",
		Preset:   "./preset.ini",
		Params:   []Param{{Key: "jinja", Value: "1"}, {Key: "temp", Value: "1.0"}, {Key: "no-mmap", Value: ""}},
		Env:      []EnvVar{{Key: "AWS_PROFILE", Value: "dev"}, {Key: "AWS_REGION", Value: "eu-west-2"}},
	}
	out := Format(sel)
	if !strings.HasPrefix(out, "PROVIDER openrouter\n") {
		t.Errorf("export not canonical:\n%s", out)
	}
	if !strings.Contains(out, "PARAM    jinja = 1") {
		t.Errorf("Format should emit PARAM lines, got:\n%s", out)
	}
	if !strings.Contains(out, "PARAM    no-mmap\n") {
		t.Errorf("Format should render an empty-valued PARAM bare, got:\n%s", out)
	}
	// PARAM sits after PRESET and before ENV in the canonical order.
	presetAt, paramAt, envAt := strings.Index(out, "PRESET"), strings.Index(out, "PARAM"), strings.Index(out, "ENV")
	if !(presetAt >= 0 && paramAt > presetAt && envAt > paramAt) {
		t.Errorf("Format should order PRESET before PARAM before ENV, got:\n%s", out)
	}
	if !strings.Contains(out, "ENV      AWS_PROFILE=dev") {
		t.Errorf("Format should emit ENV lines, got:\n%s", out)
	}
	if !strings.Contains(out, "PARALLEL 2") {
		t.Errorf("Format should emit PARALLEL, got:\n%s", out)
	}
	got, err := Parse([]byte(out))
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if !reflect.DeepEqual(got, sel) {
		t.Errorf("round-trip changed selection: %+v -> %+v", sel, got)
	}
}

func TestParse_Parallel(t *testing.T) {
	sel, err := Parse([]byte("PROVIDER llamacpp\nMODEL gemma\nCONTEXT 128k\nPARALLEL 2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if sel.Parallel != "2" {
		t.Errorf("Parallel = %q, want \"2\"", sel.Parallel)
	}
	if out := Format(sel); !strings.Contains(out, "PARALLEL 2") {
		t.Errorf("Format should emit PARALLEL, got:\n%s", out)
	}
	if _, err := Parse([]byte("PROVIDER x\nMODEL m\nPARALLEL 1\nPARALLEL 2\n")); err == nil {
		t.Error("duplicate PARALLEL should error")
	}
}

func TestFormat_NoParallelOmitsLine(t *testing.T) {
	sel := Selection{Provider: "llamacpp", Model: "gemma", Context: "128000"}
	out := Format(sel)
	if strings.Contains(out, "PARALLEL") {
		t.Errorf("Format should omit PARALLEL when unset, got:\n%s", out)
	}
}

// REMOTE was removed from the grammar: the environment is named with
// `remote deploy --node <name>` at deploy time and `--node <name>` at the
// commands that act on it, so a REMOTE line is rejected with a message that
// names the replacement.
func TestParse_RemoteRejected(t *testing.T) {
	_, err := Parse([]byte("PROVIDER openai-compatible\nREMOTE ./remote.json\n"))
	if err == nil {
		t.Fatal("a REMOTE line should be rejected")
	}
	for _, want := range []string{"line 2", "REMOTE", "removed", "--node"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should contain %q, got %q", want, err)
		}
	}
	if _, err := Parse([]byte("PROVIDER x\nremote a\n")); err == nil {
		t.Error("a lower-case remote line should be rejected too")
	}
}

func TestParse_RemoteRejectionIsNotTheGenericError(t *testing.T) {
	_, err := Parse([]byte("PROVIDER x\nREMOTE a\n"))
	if err == nil {
		t.Fatal("a REMOTE line should be rejected")
	}
	if strings.Contains(err.Error(), "unknown keyword") {
		t.Errorf("the REMOTE rejection should name the removal, not be the generic unknown-keyword error, got %q", err)
	}
}

// FLEET is no longer a keyword: a Spinloop still carrying one fails with the
// standard unknown-keyword error, which names the offending line and lists the
// accepted keywords. There is no migration message of its own.
func TestParse_FleetIsAnUnknownKeyword(t *testing.T) {
	_, err := Parse([]byte("PROVIDER llamacpp\nMODEL qwen3-27b\nFLEET ./fleet.yaml\n"))
	if err == nil {
		t.Fatal("a Spinloop carrying FLEET should not parse")
	}
	for _, want := range []string{"line 3", "FLEET", "expected PROVIDER"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should contain %q, got %q", want, err)
		}
	}
}

// The unknown-keyword error lists the accepted keywords, PARAM among them.
func TestParse_UnknownKeywordNamesParam(t *testing.T) {
	_, err := Parse([]byte("PROVIDER x\nWIDGET w\n"))
	if err == nil {
		t.Fatal("an unknown keyword should error")
	}
	if !strings.Contains(err.Error(), "PRESET, PARAM, or ENV") {
		t.Errorf("error should name PARAM among the accepted keywords, got %q", err)
	}
}

// PARAM's value may hold spaces, so it is carved from the rest of the line
// rather than fielded like every other instruction.
func TestParse_ParamValueFromRestOfLine(t *testing.T) {
	sel, err := Parse([]byte("PROVIDER llamacpp\nPARAM chat-template = {\"a\": 1} = {\"b\": 2}\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []Param{{Key: "chat-template", Value: `{"a": 1} = {"b": 2}`}}
	if !reflect.DeepEqual(sel.Params, want) {
		t.Errorf("Params = %+v, want %+v", sel.Params, want)
	}
}
