# Plan approval and operational mapping publication

A floor-plan asset represents layout geometry. A bookable space has a stable
identity and effective workplace definition. Approving a new drawing does not
itself change which spaces can be booked. This agreed ownership contract applies
to all five repository architecture candidates; concrete endpoint and transaction
design remain engineering work.

## Consumers and outcomes

| Consumer | Contract |
| --- | --- |
| Administrator portal | Review and approve an immutable asset; explicitly publish its space mappings and inspect dated-inventory impact. One workflow can perform both when validation succeeds. |
| Staff and mobile-web portal | Read the active operational mapping and its referenced approved revision, rather than treating the most recently approved drawing as active inventory. |
| SDKs, scripts and user-context MCP | Use the same authorised approval, impact and mapping-publication contracts as the portals. |
| Tracing and asset workers | Produce and verify immutable revisions through Plans. Completing a job does not grant authority to activate workplace definitions. |
| Booking and allocation workers | Validate the effective Workplace mapping/definition revision and dated inventory before committing capacity. |

## Ownership and sequence

1. Plans ingests, traces, reviews and approves an immutable asset revision. Asset
   metadata, review state, durability verification and asset pointers remain
   Plans-owned.
2. Workplace prepares the geometry-to-stable-space mapping for a floor. It owns
   the mapping revision and the operational association with the approved asset.
   Replacing geometry does not create new identities for unchanged spaces.
3. Booking validates the proposed mapping and definition impact against dated
   inventory and confirmed claims. Workplace cannot write claims, and Plans
   cannot activate inventory through a drawing-status update.
4. Workplace activates the validated mapping using the definition-version and
   Booking revalidation protocol. Public reads identify the active mapping and
   asset revision. Concurrent booking or publication must not invalidate the
   impact decision unnoticed.

The administrator may request approval followed by activation in one workflow.
These are distinct outcomes: an approved asset can remain awaiting activation if
the operational proposal fails validation or conflicts with newer state. Return
the actual outcome and, for asynchronous work, its durable operation status.
Do not report activation merely because asset processing or approval succeeded.

Operational activation preserves existing commitments unless an independently
authorised booking workflow changes them. Retirement and reactivation follow the
[workplace lifecycle contract](workplace-lifecycle.md). Asset durability, blob
recovery and referenced-revision retention continue to apply independently.

## Consistency and validation

Every candidate must specify which version/guard protects Workplace activation
against Booking commits, how dated inventory becomes usable, and how retries
recover the same result. An impact precheck alone is insufficient. The accepted
ownership does not prescribe a distributed transaction, a new service or a
particular repository layout.

Exercise approval without activation, successful combined workflow, invalid
space associations, commitment-impact refusal, concurrent claim/publication,
stale mapping revisions, retried activation and staff reads during processing.
The active mapping must reference an approved, durably retrievable asset.

See the [decision](../../adr/0030-separate-plan-approval-from-operational-mapping-activation.md)
and [API catalogue](README.md) for the surrounding contracts.
