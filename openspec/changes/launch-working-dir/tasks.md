# Tasks

## 1. Flag and launch

- [x] 1.1 Register `-w`/`--working-dir` on `openLaunchCmd`
- [x] 1.2 Resolve the path to absolute and check it exists and is a directory, before any apply
- [x] 1.3 Pass it to `launchAgent` and set the harness process's directory

## 2. Completion

- [x] 2.1 Add `--working-dir`/`-w` to `launchValueFlags`
- [x] 2.2 Offer directories only for a detached `-w`/`--working-dir` and for `--working-dir=`
- [x] 2.3 Register the flag with `compRegister` so the no-values default does not apply

## 3. Tests

- [x] 3.1 The harness runs in the named directory; without the flag it runs in spinloop's directory
- [x] 3.2 A missing path and a file path fail before the config is written
- [x] 3.3 `-O` with `-w` applies `./Spinloop` from spinloop's directory
- [x] 3.4 `-w` is not forwarded to the harness
- [x] 3.5 Completion: directories for `-w <TAB>`, `--working-dir <TAB>`, `--working-dir=<TAB>`; the Spinloop slot after `-w <dir>`; the flag is offered in both forms

## 4. Docs

- [x] 4.1 Document the flag in `docs/commands/harness.md` (`harness open`) and `docs/commands/code.md`
