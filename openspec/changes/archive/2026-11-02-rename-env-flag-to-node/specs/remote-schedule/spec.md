# Spec Delta

## MODIFIED Requirements

### Requirement: The schedule command sets, shows and clears schedules

`spinloop cloud schedule set` SHALL accept one or more `--start CRON` and `--stop CRON` flags and an optional `--timezone ZONE`, and SHALL replace the environment's schedules with them. `spinloop cloud schedule show` SHALL print the environment's schedules, one per line, giving the action, expression and time zone, and SHALL print that there are none when the list is empty. `spinloop cloud schedule clear` SHALL remove every schedule. The commands SHALL select the environment as the other `cloud` subcommands do: `--node <name>` or the per-user default. `set` with neither `--start` nor `--stop` SHALL fail and say to use `clear` to remove schedules. When the deployment's control plane has no schedule endpoint, the commands SHALL fail with an error naming the fix (re-run `spinloop cloud bootstrap`).

#### Scenario: Setting two schedules from the command line

- **WHEN** the user runs `spinloop cloud schedule set --start "0 8 * * 1-5" --stop "0 18 * * 1-5" --timezone Europe/London`
- **THEN** the environment has those two schedules and the command prints them

#### Scenario: Showing no schedules

- **WHEN** the user runs `spinloop cloud schedule show` for an environment with none
- **THEN** the output says there are no schedules and the command succeeds

#### Scenario: Clearing

- **WHEN** the user runs `spinloop cloud schedule clear`
- **THEN** the environment has no schedules and none fires afterwards

#### Scenario: Set with nothing to set

- **WHEN** the user runs `spinloop cloud schedule set` with no `--start` or `--stop`
- **THEN** the command fails and tells the user to use `clear` to remove schedules

#### Scenario: An older control plane

- **WHEN** the deployment's configuration has no schedule endpoint
- **THEN** the command fails with an error telling the user to re-run `spinloop cloud bootstrap`
