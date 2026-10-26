# Spec Delta

## ADDED Requirements

### Requirement: Generation speed from engine output

The system SHALL parse a generation speed — tokens per second — from the engine's own captured output lines. An engine that reports the figure in its structured log, llama.cpp's `print_timing` lines carrying `tg = 3.87 t/s` today, yields it; the figure read SHALL be the instantaneous one the line names for the current rate, and a line's other rate figures — a trailing three-second average, prompt-evaluation speed — SHALL not be read as it.

The speed is an optional stat on the same terms as every other: an engine whose metrics carry no such figure and whose output carries no timing line, and an engine run where its output is not captured, report the speed absent rather than reported as zero.

#### Scenario: A timing line yields the speed

- **WHEN** a captured engine line carries `tg =  12.34 t/s`
- **THEN** the collected reading carries a generation speed of 12.34 tokens per second

#### Scenario: An average is not the current rate

- **WHEN** a captured line carries only a `tg_3s = 5.29 t/s` figure, or a prompt-evaluation rate, with no instantaneous `tg` figure
- **THEN** no speed is yielded from that line

#### Scenario: An engine without timing lines reports none

- **WHEN** metrics are collected while the running engine's output carries no timing line
- **THEN** the result omits the generation speed rather than reporting zero
