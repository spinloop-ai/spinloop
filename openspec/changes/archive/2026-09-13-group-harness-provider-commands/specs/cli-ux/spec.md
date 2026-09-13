# Delta: cli-ux

## MODIFIED Requirements

### Requirement: An error names what to do about it

An error SHALL be a lowercase phrase with no trailing full stop. It SHALL quote
the value the operator supplied, name the file, flag or environment variable
it concerns, and — where a command would fix it — give that command.

An error SHALL be reported without a usage dump: the failure is what the
operator needs to read, and burying it under the command's full help hides it.

#### Scenario: An unknown name says what the known ones are

- **WHEN** the operator names something that does not exist
- **THEN** the error quotes what they typed, says where it was looked for, and
  either lists what is there or names the command that would

#### Scenario: A broken reference names its repair

- **WHEN** a stored reference points at something that has gone
- **THEN** the error says what it points at and gives the command that would
  re-point or remove it

#### Scenario: A moved command names its new home

- **WHEN** the operator runs a command at a spelling that no longer exists
  because the command moved to a group
- **THEN** the error names the command's new home, giving the new spelling,
  rather than a bare unknown-command message
