# Separate Events from venue booking

Events owns event details, registration, tickets and admission, while Booking
owns the event-space booking and its capacity claims; keeping Events separate
preserves its distinct attendance lifecycle instead of broadening Booking to own
the entire event operation. Venue changes affecting ticket capacity and ticket
issuance coordinate validation through the owners' published interfaces in one
local PostgreSQL transaction where required, accepting explicit locking and
transaction coordination rather than allowing stale checks or foreign-repository
access. This settles the Events–Booking boundary without approving the rest of
the proposed module map.
