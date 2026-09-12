package hf

import (
	"strings"
	"testing"
)

func TestParseRef(t *testing.T) {
	cases := []struct {
		in  string
		ref Ref
		err string // substring the error must contain; "" means no error
	}{
		{
			in:  "unsloth/Qwen3.6-35B-A3B-GGUF",
			ref: Ref{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "main"},
		},
		{
			in:  "unsloth/Qwen3.6-35B-A3B-GGUF:Q4_K_M",
			ref: Ref{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "main", Quant: "Q4_K_M"},
		},
		{
			in:  "unsloth/Qwen3.6-35B-A3B-GGUF@main:Q4_K_M",
			ref: Ref{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "main", Quant: "Q4_K_M"},
		},
		{
			in:  "unsloth/Qwen3.6-35B-A3B-GGUF@e1d7ec4",
			ref: Ref{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "e1d7ec4"},
		},
		{
			in:  "unsloth/Qwen3.6-35B-A3B-GGUF@v0.9",
			ref: Ref{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "v0.9"},
		},
		{
			in:  "hf.co/unsloth/Qwen3.6-35B-A3B-GGUF",
			ref: Ref{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "main"},
		},
		{
			in:  "huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF:Q8_0",
			ref: Ref{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "main", Quant: "Q8_0"},
		},
		{
			in:  "https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF",
			ref: Ref{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "main"},
		},
		{
			in:  "https://hf.co/unsloth/Qwen3.6-35B-A3B-GGUF",
			ref: Ref{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "main"},
		},
		{
			in:  "https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF/tree/main",
			ref: Ref{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "main"},
		},
		{
			in:  "https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF/tree/e1d7ec4:Q8_0",
			ref: Ref{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "e1d7ec4", Quant: "Q8_0"},
		},
		{
			in:  "https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF@main",
			ref: Ref{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "main"},
		},
		{
			in:  "https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF/tree/main@main",
			ref: Ref{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "main"},
		},
		{
			in: "https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF/blob/main/Qwen3.6-35B-A3B-UD-Q4_K_XL.gguf",
			ref: Ref{
				Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "main",
				File: "Qwen3.6-35B-A3B-UD-Q4_K_XL.gguf",
			},
		},
		{in: "  unsloth/Qwen3.6-35B-A3B-GGUF  ", ref: Ref{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "main"}},
		{
			in:  "http://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF",
			ref: Ref{Owner: "unsloth", Name: "Qwen3.6-35B-A3B-GGUF", Revision: "main"},
		},

		// Failures.
		{in: "", err: "org/model"},
		{in: "   ", err: "org/model"},
		{in: "qwen3.6", err: "org/model"},
		{in: "unsloth", err: "org/model"},
		{in: "unsloth/Qwen3.6-35B-A3B-GGUF:Q4_K_M@main", err: "org/model"},
		{in: "a/b/c", err: "org/model"},
		{in: "hf.co/unsloth", err: "org/model"},
		{in: "example.com/unsloth/model", err: "org/model"},
		{in: "https://example.com/unsloth/Qwen3.6-35B-A3B-GGUF", err: "not a Hugging Face URL"},
		{in: "ftp://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF", err: "not a Hugging Face URL"},
		{in: "https://huggingface.co/unsloth", err: "path"},
		{in: "https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF/tree", err: "path"},
		{in: "https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF/tree/main/extra", err: "path"},
		{in: "https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF/discussions/12", err: "path"},
		{
			in:  "https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF/blob/main/Q4.gguf:Q4_K_M",
			err: "names both a file and a :QUANT",
		},
		{
			in:  "https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF/tree/e1d7ec4@main",
			err: "two different revisions",
		},
		{in: "unsloth/Qwen3.6-35B-A3B-GGUF@", err: "org/model"},
		{in: "unsloth/Qwen3.6-35B-A3B-GGUF:", err: "org/model"},
	}

	for _, tc := range cases {
		name := tc.in
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			got, err := ParseRef(tc.in)
			if tc.err != "" {
				if err == nil {
					t.Fatalf("ParseRef(%q) = %+v, want error containing %q", tc.in, got, tc.err)
				}
				if !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("ParseRef(%q) error = %q, want it to contain %q", tc.in, err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRef(%q) error: %v", tc.in, err)
			}
			if got != tc.ref {
				t.Errorf("ParseRef(%q) = %+v, want %+v", tc.in, got, tc.ref)
			}
		})
	}
}
