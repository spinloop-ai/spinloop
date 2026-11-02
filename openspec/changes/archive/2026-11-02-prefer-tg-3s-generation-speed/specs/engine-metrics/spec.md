# Spec Delta

## Modified Requirements

### Requirement: Generation speed from engine output

The system SHALL parse a generation speed — tokens per second — from the engine's own captured output lines. An engine that reports the figure in its structured log, llama.cpp's `print_timing` lines carrying `tg = 3.87 t/s` and `tg_3s = 5.29 t/s` today, yields it. The figure read SHALL be the line's three-second rate (`tg_3s`) where the line carries a usable one, and SHALL fall back to the instantaneous rate (`tg`) where it carries none or the three-second figure is not usable — the shorter window answers "how fast is it generating now", which is the figure the speed is for, while `tg` averages the whole task and lags a rate that has changed. Rates the line carries under other labels — prompt-evaluation speed among them — SHALL not be read as it.

The speed is an optional stat on the same terms as every other: an engine whose metrics carry no such figure and whose output carries no timing line, and an engine run where its output is not captured, report the speed absent rather than reported as zero.

#### Scenario: A timing line yields the speed

- **WHEN** a captured engine line carries `tg =  12.34 t/s` and no `tg_3s` figure
- **THEN** the collected reading carries a generation speed of 12.34 tokens per second

#### Scenario: An average is not the current rate

- **WHEN** a captured line carries both `tg = 3.87 t/s` and `tg_3s = 5.29 t/s`
- **THEN** the collected reading carries 5.29 tokens per second — the three-second rate, and not the whole-task average `tg` names

#### Scenario: An engine without timing lines reports none

- **WHEN** metrics are collected while the running engine's output carries no timing line
- **THEN** the result omits the generation speed rather than reporting zero

#### Scenario: A line without an instantaneous rate still yields the speed

- **WHEN** a captured line carries only a `tg_3s = 5.29 t/s` figure
- **THEN** the collected reading carries a generation speed of 5.29 tokens per second

#### Scenario: An unusable three-second figure falls back

- **WHEN** a captured line carries an unusable `tg_3s` figure — zero or an unparsable number — beside a usable `tg` figure
- **THEN** the collected reading carries the `tg` rate

#### Scenario: Other rates yield nothing

- **WHEN** a captured line carries a prompt-evaluation rate with no `tg` or `tg_3s` figure
- **THEN** no speed is yielded from that line
