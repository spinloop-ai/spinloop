# spinloop completion

Tab completion for bash, zsh, and PowerShell.

```sh
source <(spinloop completion bash)   # add to ~/.bashrc
source <(spinloop completion zsh)    # or ~/.zshrc (needs compinit)
spinloop completion powershell | Out-String | Invoke-Expression   # or $PROFILE
```

Homebrew installs the bash and zsh completions for you.

## What completes

TAB completes commands, flags, and — context-aware — the values that follow
them:

- provider names after `-p` (honouring a `--providers` override on the line)
- harness names after `-H`, `--harness`, or `--set`
- your [registered aliases](alias.md) wherever a Spinloop path goes —
  `spinloop unalias <TAB>` offers exactly the names you have
- the supported shells after `completion`

## See also

- [`spinloop alias`](alias.md) — the names completion offers
