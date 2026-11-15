# Limit Domain Glossary

## Unit

The measurement category of a limit, identified externally by an immutable code such as `BALANCE` or `AMOUNT`. An inactive unit remains administratively visible but excludes its limit types and rules from resolved configurations.

## Limit Type

The immutable coded meaning of a limit, assigned permanently to one unit. Its description and active status may change. An inactive limit type remains administratively visible but its rules are excluded from resolved configurations.

## Limit Group

The immutable combination of wallet type, ISO 4217 currency, and tier that owns system limits. A group can be created, read, or hard-deleted, but not updated or listed globally.

## System Limit

A rule belonging to a limit group. Its identity is the group, limit type, transaction type, and channel; its operator and value may change. Resolved API rules identify it with source `SYSTEM`.

## User Limit

A wallet-scoped rule that can replace an exactly matching system-limit identity or add a more specific rule while its effective window is active. Resolved API rules identify it with source `USER`.

## Explicit Audit Scope

Only the entity directly targeted by a create, update, or delete API operation produces an audit record. Rows removed indirectly by database cascades do not produce additional audit records.

