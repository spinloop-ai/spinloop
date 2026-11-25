## ADDED Requirements

### Requirement: Choosing the harness's working directory

`spinloop harness open` and `spinloop code` SHALL accept
`-w`/`--working-dir <dir>` and SHALL run the launched harness in that
directory, resolving a relative path against spinloop's own working directory.
Without the flag, the harness SHALL run where spinloop runs. The flag SHALL
NOT change where spinloop finds `./Spinloop`, `./fleet.yaml`, or a `.env`.

#### Scenario: The harness runs in the named directory

- **WHEN** the user runs `spinloop code -w ../other-project`
- **THEN** the harness is launched with `../other-project`, resolved against
  the current directory, as its working directory

#### Scenario: Without the flag the harness runs where spinloop ran

- **WHEN** the user runs `spinloop code` with no `-w`
- **THEN** the harness is launched in the directory spinloop was run from

#### Scenario: spinloop's own lookups are unchanged

- **WHEN** the user runs `spinloop code -O -w ../other-project`
- **THEN** the `Spinloop` applied is the one in the directory spinloop was run
  from, not in `../other-project`, and the harness runs in
  `../other-project`

#### Scenario: The flag is spinloop's, not the harness's

- **WHEN** the user runs `spinloop code -w ../p run --continue`
- **THEN** the harness is invoked with `run --continue` and runs in `../p`;
  `-w ../p` is not forwarded

### Requirement: An invalid working directory is refused

When the path given to `--working-dir` does not exist or is not a directory,
the launch SHALL fail with an error naming the path, before any Spinloop is
applied or any harness config is written.

#### Scenario: A missing directory fails before applying

- **WHEN** the user runs `spinloop code -w ./missing qwen` and `./missing`
  does not exist
- **THEN** the command fails naming `./missing`, and the harness config is not
  changed

#### Scenario: A file is not a working directory

- **WHEN** the path given to `-w` is a regular file
- **THEN** the command fails saying the path is not a directory
