# Create event visitor visits with tickets atomically

When registration creates a visit specifically for an event ticket, Events and
Visitors commit the new visit, ticket and retry outcome in one local transaction
through their owning interfaces, so losing the last ticket or failing a visit
check leaves no unexpected visit. Reusing an existing valid visit does not make
that visit subject to rollback or deletion when ticket issuance fails; this
accepts narrow transaction coordination instead of staged orphan-visit cleanup.
A definitive refusal retains its required retry outcome without creating a new
visit or ticket.
