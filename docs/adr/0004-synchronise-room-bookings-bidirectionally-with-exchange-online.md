# Synchronise room bookings bidirectionally with Exchange Online

Room bookings, changes and cancellations originating in SpaceWeave update
Microsoft Exchange Online, and those originating in Exchange Online update
SpaceWeave. Both booking entry points participate in room availability checks.
This lets staff use either booking workflow while keeping room schedules aligned.

Use a dedicated backend Exchange identity with scoped mailbox access,
notifications and durable delta reconciliation on a five-minute cadence while
healthy. Persist progress and monitor actual synchronisation freshness.
