# Design

## Context

`launchAgent` (`cmd/spinloop/commands.go`) builds the agent's environment from
a resolver: `.env` and the process environment, extended with a cloud
endpoint's key (`cloudLaunchResolver`). v1.51.0 added `storedKeyResolver`,
which wraps that resolver with `catalog.Provider.KeyResolver` for the applied
Spinloop's provider. It returns the resolver unchanged when no Spinloop is
applied. `harnessEnv` and `lucinateLaunchKey` read every key through the
wrapped resolver.

Every harness reports its config through `Harness.State()`: each provider's
base URL, and the default model as `<provider>/<model>` for opencode, or `""`
for Pi and lucinate.

## Goals / Non-Goals

**Goals:**

- A bare `spinloop code` gets the stored key of the provider the harness will
  use, as a launch that applies a Spinloop does.
- Keep the change to the launch's key lookup, away from the cloud code that
  PR 286 refactors.

**Non-Goals:**

- Fetching a cloud node's key on a bare launch. A cloud provider's id in the
  harness config is the node name, which is not a catalogue id, so it gets no
  stored key. The cloud path stays as it is.
- Restarting an opencode background service started without the key.

## Decisions

**Choosing the provider.** `storedKeyResolver` keeps its signature, plus the
harness:

- Applied Spinloop (`worn`): as before, the selection's provider, at the
  endpoint the apply wrote, and nothing for cloud or fleet launches.
- Otherwise: call `h.State()`. If it reports a default model, take the text
  before the first `/` as the provider id. If it reports none, use every
  provider id in the state.

For each id that is in the catalogue and declares `apiKeyFile`, wrap the
resolver with `p.KeyResolver(resolve, endpoint)`. Each wrapper answers only its
own provider's key variable, so the wrappers chain safely. A `State()` error
leaves the resolver unchanged, so a launch never fails over a key it could not
look up.

**Endpoint.** The base URL `State()` holds for the provider. When that is
empty, use `discovery.ResolveBaseURL(p, "", resolve)`, which is the
catalogue's endpoint, overridden by `SPINLOOP_BASE_URL` or the provider's base
URL variable. This is the same function the applied path uses.

**Where it plugs in.** `launchAgent` already calls `storedKeyResolver` before
`harnessEnv` and `lucinateLaunchKey`. Only the helper and its call change, so
both consumers get the key with no further edits.

**opencode references the key file.** opencode substitutes `{file:<path>}`
as well as `{env:VAR}` in its config, including in a provider's `apiKey`. A
prototype against Unsloth Studio with `UNSLOTH_API_KEY` unset showed:

- a standalone run with `{file:<Studio key file>}` authenticates, and opencode
  strips the trailing newline Studio writes;
- the same run with `{env:UNSLOTH_API_KEY}` fails with "Not authenticated";
- a long-lived `opencode serve` started with no key in its environment, on the
  `{file:}` config, authenticates requests from a separate client;
- a running server does not reload its config, so switching from `{env:}` to
  `{file:}` takes effect after one restart.

`BuildProviderBlock` resolves the key variable as before. When the variable is
empty, the provider declares `apiKeyFile`, and the endpoint is local, it writes
`{file:<path>}`, using the path of the newest matching file
(`(*Provider).StoredKeyPath`), provided that file holds a key. The path is
absolute after `$HOME` is expanded, so the config does not depend on how
opencode expands `~`. The key prefix check applies to the file's contents as
it does to a variable. Pi and lucinate keep their current references: neither
supports file references, and lucinate already receives its key at launch.

## Risks / Trade-offs

- The file path is fixed when the config is applied. If Studio stores its key
  under a new name, the config points at a file that no longer exists until
  the Spinloop is applied again.

- opencode's default model id is assumed to be `<provider>/<model>`. If it is
  not, no provider is found, and the launch behaves as v1.51.0 does.
- For Pi and lucinate with no Spinloop, every configured provider with a key
  file is considered. Each one still has to be local and have its variable
  unset, so at most the keys of local servers on this machine are passed on.
- PR 286 will conflict on nearby comments in `commands.go` at most. The
  helper sits below `launchAgent`, which #286 does not change.
