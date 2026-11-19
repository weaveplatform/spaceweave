# Commit retry receipts with command acceptance

Booking creation, ticket issuance and background-work submissions require a retry
key, with the accepted outcome/receipt committed in the same PostgreSQL transaction
as the business change or durable work submission. This lets a client recover the
existing outcome after a lost response or server restart; a response cache saved
after the business commit would leave a gap in which the command could execute
again. External provider effects still occur after commit and retain their own
deduplication and reconciliation rules.
