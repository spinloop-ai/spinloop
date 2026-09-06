## ADDED Requirements

### Requirement: Sparkline never draws a full block

The sparkline SHALL map a series' 0–100% values to Unicode block elements of one
grade per utilisation level, capped at the seven-eighths block (`▇`): the highest
value in a series renders as `▇`, and no value renders as the full block (`█`).
Capping the tallest glyph one grade short of full height leaves the top eighth of
each cell unfilled, so two adjacent series at their maximum read as separate bars
with a visible gap between the bottom of one row and the top of the next, instead
of merging into a single solid block. The exact value is carried by the trailing
percentage, so a glyph may sit one grade below full height without losing
information.

#### Scenario: A value at the top of the range caps at seven-eighths

- **WHEN** a sample in a series reaches 100%
- **THEN** its glyph is the seven-eighths block (`▇`), not the full block (`█`)

#### Scenario: No sparkline glyph is a full block

- **WHEN** the sparkline draws any series from its retained samples
- **THEN** none of the glyphs it draws is the full block (`█`)

#### Scenario: Adjacent maxed rows keep a visible gap

- **WHEN** two adjacent series are both at their maximum across the window
- **THEN** each row's tallest glyph leaves the top eighth of its cell unfilled, so a blank strip separates the rows rather than the rows reading as one solid block
