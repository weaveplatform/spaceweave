# Restrict recurring room bookings and cancel unused series

Recurring room authoring and management require an explicitly granted capability;
the organiser or an authorised attendee can establish occurrence use by checking in.
Booking counts consecutive unused occurrences against a configurable threshold,
resets the count after verified use, and cancels the remaining meetings in
Exchange with attendee notification when the threshold is reached.
This reclaims abandoned room capacity at the cost of operational usage validation
and durable provider reconciliation.
