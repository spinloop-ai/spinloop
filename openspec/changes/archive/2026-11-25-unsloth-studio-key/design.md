# Design

## Context

Provider keys are resolved through a `resolve func(string) string` built by
`opencode.EnvResolver(dir)`: the process environment, then the `.env` in the
Spinloop's directory. The same function is passed to the opencode, Pi and
lucinate builders in `internal/catalog`, to `discovery.Models` for tab
completion, and to `launchAgent`. `launchAgent` puts every catalogue key that
resolves into the harness's environment (`harnessEnv`) and works out lucinate's
launch key (`lucinateLaunchKey`).

`unsloth run` stores the key it generated in
`~/.unsloth/studio/auth/.cli_api_key_<name>_<id>`, where `<name>` is the
`--api-key-name` label (default `cli`) and `<id>` is a suffix Studio picks. The
file holds the key followed by a newline. spinloop starts `unsloth run` without
`--api-key-name`, so the label is `cli`.

## Goals / Non-Goals

**Goals:**

- A local Studio's key reaches the harness with nothing exported.
- The existing precedence stays: `ENV` > process environment > `.env`, and the
  key file only fills a gap after all three.
- No secret is written to disk by spinloop.

**Non-Goals:**

- Choosing Studio's key, or passing one to `unsloth run`. Studio has no option
  for that.
- Restarting a detached opencode service that was started without the key.
  opencode reads `{env:…}` from the service's own environment, so a service
  started before the key was available keeps failing until it is restarted.
  This is documented, not handled.
- Keys stored under a non-default `--api-key-name`.

## Decisions

**A catalogue field, not Go code naming Studio.** `apiKeyFile` sits next to
`apiKeyEnv` in `providers.yaml`, so where a key is stored is data, as the
catalogue's other per-provider details are. A custom catalogue can use it for
any other local server that stores its key in a file.

**One wrapper over `resolve`.** `(*Provider).KeyResolver(resolve, baseURL)`
returns a function that answers `p.APIKeyEnv` from `resolve` first. When that
is empty, the endpoint is local, and `p.APIKeyFile` is set, it returns the
stored key. Any other name is passed straight to `resolve`. Callers that already
take a `resolve` function wrap it at the point where they know the base URL:

- `BuildProviderBlock` and `BuildPiProvider`, after the base URL is worked out.
  The existing "optional key, local endpoint, unset" branches then see a set
  key, and the configs get the env reference.
- `discovery.Models`, so tab completion can list a local Studio's models.
- `launchAgent`, for the worn selection's provider. The base URL is the one the
  apply used: the selection's `BASEURL`, else `SPINLOOP_BASE_URL`, else the
  provider's own endpoint. The wrapped function feeds `harnessEnv` and
  `lucinateLaunchKey`, so the key reaches the child under `UNSLOTH_API_KEY`
  through the code path that sets every other key. Launches through a cloud
  endpoint or a fleet are not wrapped: their key comes from the endpoint.

`.env` precedence holds because `resolve` already checks the process
environment and `.env` before the wrapper looks at the file. An `ENV`
instruction still overrides everything, because `overlayLocalEnv` runs after
`harnessEnv`.

**Local endpoints only.** The file belongs to the server on this machine.
Sending it to a remote host would hand a local secret to whichever host
`UNSLOTH_BASE_URL` names. `IsLocalEndpoint` is the existing test for this.

**Newest file wins.** The `<id>` suffix is not documented. If Studio ever leaves
more than one file, the one written last belongs to the most recent run. Files
that cannot be read are skipped.

**The pattern is expanded with `os.ExpandEnv`, then `filepath.Glob`.** This
matches how `$HOME` is written in the rest of the catalogue and in Spinloop
files. A malformed pattern yields no match, so the key is unset.

## Risks / Trade-offs

- Studio may change where or how it stores the key. Then no file matches, and
  the behaviour is the same as before this change, with the user exporting the
  key. The catalogue entry is the only thing to update.
- `apply` with no launch now writes an env reference where it used to leave the
  key out. A harness started outside spinloop without `UNSLOTH_API_KEY` sends an
  empty key, so a Studio that needs a key returns 401 just as it did before.
  A Studio with keyless access turned on ignores the key.
