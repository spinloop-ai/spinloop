# Spec Delta

## MODIFIED Requirements

### Requirement: The metrics report's tabular display

The stats output SHALL support four formats via the `--format` flag: `gauge` (default), `bar`, `table`, and `json`. The `gauge` format SHALL produce a compact display with horizontal progress gauges for the current reading, colour-coded by utilization level. The `bar` format SHALL produce a compact display drawing each resource series as a sparkline of the daemon's retained history, with each point colour-coded by its own utilization level. The `table` format SHALL produce a tab-separated key-value table, one line per metric, with the key column left-aligned and values right of it. The `json` format SHALL output the response as a JSON object to standard output. Progress and error messages SHALL go to standard error regardless of format.

#### Scenario: Clean output

- **WHEN** the command succeeds
- **THEN** standard output contains only the stats data with no progress or debug lines

#### Scenario: Default format is gauge

- **WHEN** the user runs `spinloop metrics --env <name>` without `--format`
- **THEN** the output is in gauge format

#### Scenario: Table format is explicit

- **WHEN** the user runs `spinloop metrics --env <name> --format=table`
- **THEN** the output is in table format

#### Scenario: Bar format is explicit

- **WHEN** the user runs `spinloop metrics --env <name> --format=bar`
- **THEN** the output is in bar format, drawing each resource series as a sparkline of the daemon's retained history

#### Scenario: Gauge format is explicit

- **WHEN** the user runs `spinloop metrics --env <name> --format=gauge`
- **THEN** the output is in gauge format with progress gauges for the current reading

#### Scenario: JSON format

- **WHEN** the user runs `spinloop metrics --env <name> --format=json`
- **THEN** the output is valid JSON containing the instance state, runner, model, GPU info, CPU/RAM usage, and token counts

#### Scenario: JSON format with cost

- **WHEN** the user runs `spinloop metrics --env <name> --format=json --cost`
- **THEN** the JSON output includes a cost estimate field

#### Scenario: Invalid format errors

- **WHEN** the user runs `spinloop metrics --env <name> --format=csv`
- **THEN** the command exits with an error and usage message
