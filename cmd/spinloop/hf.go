package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/spinloop-ai/spinloop/internal/catalog"
	"github.com/spinloop-ai/spinloop/internal/harness"
	"github.com/spinloop-ai/spinloop/internal/hf"
	"github.com/spinloop-ai/spinloop/internal/opencode"
	"github.com/spinloop-ai/spinloop/internal/spinloop"
)

// hfCmd builds the `hf` command: it reads a Hugging Face model repo and
// writes the Spinloop that serves it.
func hfCmd() *cobra.Command {
	var provider, quant, contextFlag, alias, outputFile, harnessName string
	var force, noCache, apply bool
	c := &cobra.Command{
		Use:   "hf <ref>",
		Short: "write a Spinloop for a Hugging Face model",
		Long: `reads a Hugging Face model repo — a pasted org/model, a model-page
URL, either with a :QUANT or @revision — and writes the Spinloop that
serves it: the provider inferred from the repo's files, the quantisation,
the context window the model's config declares, and a short alias. The
Spinloop goes to stdout, or to --output-file/-o, and --apply configures
the active harness straight away. What was inferred, and from what, is
said on stderr, so stdout stays a clean Spinloop.

The repo is read, never downloaded — metadata only, from the Hub or from
the caches a copy is already in. -o here names the output file; on the
other commands it is the max output tokens. This command writes no OUTPUT
line: applying one defaults the output to a quarter of the context.`,
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(c *cobra.Command, args []string) error {
			resolve(c)
			if len(args) == 0 {
				_ = c.Usage()
				return fmt.Errorf("a Hugging Face reference is required: spinloop hf org/model")
			}
			if len(args) > 1 {
				return fmt.Errorf("hf takes one reference, got %d", len(args))
			}
			ref, err := hf.ParseRef(args[0])
			if err != nil {
				return err
			}
			if provider != "" {
				if err := knownProvider(provider); err != nil {
					return err
				}
			}
			client := hf.DefaultClient()
			roots := hf.ResolveRoots()
			info, err := client.Fetch(ref.Repo(), ref.Revision, roots)
			if err != nil {
				return err
			}
			res, err := hf.Resolve(info, ref, roots, hf.Options{
				Provider: provider,
				Quant:    quant,
				Context:  contextFlag,
				Alias:    alias,
				NoCache:  noCache,
			})
			if err != nil {
				return err
			}
			sel := spinloop.Selection{
				Provider: res.Provider,
				Model:    res.Model,
				Alias:    res.Alias,
				Context:  res.Context,
			}
			rendered := spinloop.Format(sel)
			if outputFile != "" {
				if err := writeSpinloopFile(outputFile, rendered, force); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "Spinloop written to %s\n", outputFile)
			} else {
				fmt.Print(rendered)
			}
			writeReasoning(res)
			if apply {
				h, _, err := harness.Resolve(harnessName)
				if err != nil {
					return err
				}
				return applySelection(sel, h, "", opencode.EnvResolver(""))
			}
			return nil
		},
	}
	fs := c.Flags()
	fs.StringVarP(&provider, "provider", "p", "", "engine to serve it with (default: inferred from the repo's files)")
	fs.StringVarP(&quant, "quant", "q", "", "quantisation to serve (default: a documented preference, the alternatives named on stderr)")
	fs.StringVarP(&contextFlag, "context", "c", "", "context window (default: the window the model's config declares)")
	fs.StringVarP(&alias, "alias", "a", "", "short name for the model (default: the repo name, minus a packaging suffix)")
	fs.StringVarP(&outputFile, "output-file", "o", "", "write the Spinloop to this file instead of stdout")
	fs.BoolVar(&force, "force", false, "overwrite an existing --output-file")
	fs.BoolVar(&noCache, "no-cache", false, "write the repo reference even when a copy is cached, so the Spinloop stays portable")
	fs.BoolVar(&apply, "apply", false, "configure the active harness from the result")
	fs.StringVarP(&harnessName, "harness", "H", "", "which harness to configure (with --apply)")
	fs.SetInterspersed(false)
	c.ValidArgsFunction = noPositionals
	compRegister(c, "provider", compProviders)
	compRegister(c, "output-file", compFiles)
	compRegister(c, "harness", compHarnessNames)
	return c
}

// knownProvider fails a -p that names no provider in the catalogue, before
// any network call is made for nothing.
func knownProvider(name string) error {
	cat, err := catalog.LoadFrom(catalog.ResolveCatalogPath(""))
	if err != nil {
		return err
	}
	if _, ok := cat.Providers[name]; !ok {
		return fmt.Errorf("unknown provider %q (see `spinloop list`)", name)
	}
	return nil
}

// writeSpinloopFile writes the rendered Spinloop to path, refusing to
// overwrite an existing file unless force is set — a hand-edited Spinloop
// cannot be lost to a mistyped command.
func writeSpinloopFile(path, rendered string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("%s already exists — pass --force to overwrite it", path)
	}
	if err := os.WriteFile(path, []byte(rendered), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// writeReasoning says on stderr what the resolver inferred and from what —
// the provider and why, the quantisation and the alternatives, the context
// and its source, and which copy of the model was used — so stdout carries
// nothing but the Spinloop.
func writeReasoning(res hf.Resolved) {
	fmt.Fprintf(os.Stderr, "provider  %s — %s\n", res.Provider, res.ProviderWhy)
	if res.Quant != "" {
		line := "quant     " + res.Quant
		if len(res.Quants) > 1 {
			rest := make([]string, 0, len(res.Quants)-1)
			for _, q := range res.Quants {
				if q != res.Quant {
					rest = append(rest, q)
				}
			}
			if len(rest) > 0 {
				line += " — the repo also offers " + strings.Join(rest, ", ")
			}
		}
		fmt.Fprintln(os.Stderr, line)
	}
	if res.Context != "" {
		fmt.Fprintf(os.Stderr, "context   %s — %s\n", res.Context, res.ContextSource)
	} else {
		fmt.Fprintln(os.Stderr, "context   (none) — the model declares no window, so the Spinloop has no CONTEXT line")
	}
	switch {
	case res.ModelSource != "":
		fmt.Fprintf(os.Stderr, "model     %s — from %s; the engine loads it instead of downloading\n", res.Model, res.ModelSource)
	case res.CachedPath != "":
		fmt.Fprintf(os.Stderr, "model     %s — the repo reference rather than the cached path, because --no-cache was given; the copy at %s stays on this machine\n", res.Model, res.CachedPath)
	default:
		fmt.Fprintf(os.Stderr, "model     %s — not on this machine; the engine fetches what it needs on first use\n", res.Model)
	}
}

// Test seam: the suite calls this the way the tree does — it runs the
// command through execCmd rather than parsing a private FlagSet.
func cmdHf(args []string) error { return execCmd(hfCmd(), args) }
