# Separate Exchange synchronisation from Booking

Exchange owns provider mappings, subscriptions, desired/observed calendar state
and reconciliation progress in a separate module; Booking retains authority over
committed room intentions, local capacity and booking transitions.
Reconciliation consumes committed intentions and applies observations through
private owning interfaces, accepting a retry-safe handoff so provider integration
has its own lifecycle without acquiring a second claim writer. Identity retains
its separate adapter for fresh native delegation evidence because authorisation
is distinct from calendar reconciliation.
