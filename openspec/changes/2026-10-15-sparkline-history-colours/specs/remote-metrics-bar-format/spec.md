# Spec Delta

## MODIFIED Requirements

### Requirement: Colour thresholds

The sparkline's points SHALL be colour-coded based on utilization: yellow for values from 80% to 90%, and red for values above 90%. Every point SHALL take the colour of its own value, the latest point included, and every point below 80% SHALL appear in the terminal's default colour, so the threshold crossings — wherever in the window they sit — are the ones to read.

#### Scenario: A historical point above 90% stays red

- **WHEN** a series' retained history holds a sample above 90% earlier in the window and later samples fall below 80%
- **THEN** the earlier glyph appears in red and the later glyphs appear in the terminal's default colour

#### Scenario: A historical point at 80% to 90% stays yellow

- **WHEN** a series' retained history holds a sample from 80% to 90% earlier in the window and later samples fall below 80%
- **THEN** the earlier glyph appears in yellow and the later glyphs appear in the terminal's default colour

#### Scenario: The latest point takes the colour of its own value

- **WHEN** the latest sample of a series is 95%
- **THEN** the sparkline's final glyph appears in red

#### Scenario: The latest point at 80% to 90% is yellow

- **WHEN** the latest sample of a series is 85%
- **THEN** the sparkline's final glyph appears in yellow

#### Scenario: A latest point below 80% is uncoloured

- **WHEN** the latest sample of a series is 70%
- **THEN** the sparkline's final glyph appears in the terminal's default colour

#### Scenario: A window below the thresholds stays uncoloured

- **WHEN** every sample of a series is below 80%, the latest included
- **THEN** every glyph of the sparkline appears in the terminal's default colour
