## MODIFIED Requirements

### Requirement: Tabular display

The stats output SHALL support four formats via the `--format` flag: `gauge` (default), `bar`, `table`, and `json`. The `gauge` format SHALL produce a compact display with horizontal progress gauges for the current reading, colour-coded by utilization level. The `bar` format SHALL produce a compact display drawing each resource series as a sparkline of the daemon's retained history, with the latest point colour-coded by utilization level. The `table` format SHALL produce a tab-separated key-value table, one line per metric, with the key column left-aligned and values right of it. The `json` format SHALL output the response as a JSON object to standard output. Progress and error messages SHALL go to standard error regardless of format.

#### Scenario: Clean output

- **WHEN** the command succeeds
- **THEN** standard output contains only the stats data with no progress or debug lines

#### Scenario: Default format is gauge

- **WHEN** the user runs `spinloop remote metrics` without `--format`
- **THEN** the output is in gauge format

#### Scenario: Table format is explicit

- **WHEN** the user runs `spinloop remote metrics --format=table`
- **THEN** the output is in table format

#### Scenario: Bar format is explicit

- **WHEN** the user runs `spinloop remote metrics --format=bar`
- **THEN** the output is in bar format, drawing each resource series as a sparkline of the daemon's retained history

#### Scenario: Gauge format is explicit

- **WHEN** the user runs `spinloop remote metrics --format=gauge`
- **THEN** the output is in gauge format with progress gauges for the current reading

#### Scenario: JSON format

- **WHEN** the user runs `spinloop remote metrics --format=json`
- **THEN** the output is valid JSON containing the instance state, runner, model, GPU info, CPU/RAM usage, and token counts

#### Scenario: JSON format with cost

- **WHEN** the user runs `spinloop remote metrics --format=json --cost`
- **THEN** the JSON output includes a cost estimate field

#### Scenario: Invalid format errors

- **WHEN** the user runs `spinloop remote metrics --format=csv`
- **THEN** the command exits with an error and usage message

### Requirement: History in the report

When the on-instance daemon's metrics reply carries a history of system readings, the report SHALL carry it through to the command's output: the `json` format SHALL include the readings, and the `bar` format SHALL draw them. Where the daemon's reply carries no history, the report SHALL omit the field and the `bar` format SHALL fall back per the bar format specification. The control plane's relay of the daemon's reply SHALL NOT alter the readings it carries.

#### Scenario: JSON carries the daemon's history

- **WHEN** the instance's daemon reports a retained history and the user runs `spinloop remote metrics --format=json`
- **THEN** the JSON output includes the history's readings

#### Scenario: Bar draws the relayed history

- **WHEN** the instance's daemon reports a retained history and the user runs `spinloop remote metrics --format=bar`
- **THEN** each resource series is drawn as a sparkline from the readings the control plane relayed

#### Scenario: A daemon without history degrades

- **WHEN** the instance runs a daemon whose reply carries no history and the user runs `spinloop remote metrics --format=bar`
- **THEN**    the report omits the history field and bar format draws the current reading in the gauge's filled style
