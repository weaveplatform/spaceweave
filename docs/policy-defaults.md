# Operational policy defaults

Status: agreed initial defaults, 19 November 2026. Every value below is
configurable, including weekday, local time, period length and duration values.
Organisation defaults and permitted office overrides apply. Office dates and
times use the office calendar and timezone.

| Policy | Initial default | Meaning |
| --- | --- | --- |
| Intent lookahead | 28 calendar days | Future attendance dates staff can request. |
| Allocation cycle | 7 days, starting Monday | Attendance dates planned as one cycle; closed dates remain unavailable. |
| Intent cutoff | 14 calendar days before the cycle start date, at 17:00 local | Deadline for that cycle's main cohort. |
| Main allocation run | 30 minutes after cutoff | Plan and commit the cohort; handle later changes incrementally. |
| Desk no-show grace | 60 minutes after booking start | Assignee must personally check in before expiry. |
| Post-check-in vacancy | 30 minutes | Require fresh, sustained vacancy evidence; missing signals mean unknown. |
| Same-day waitlist offer | 10 minutes | Acceptance deadline before offering capacity to the next eligible person. |
| Automatic room acceptance hold | 15 minutes | Default for automatically processed rooms; approval-based rooms require an explicit longer workflow deadline. |

The delegation verification limit of fifteen minutes is already agreed and
configurable. Retention periods remain explicitly configured by the organisation
for attendance, occupancy evidence and visitor visits.

Operational policies are versioned database records editable through
administration, separate from deployment configuration. Apply changes to new
cycles/bookings by default; existing confirmations retain their policy version
unless explicitly migrated. Validate individual values and cross-field
consistency before publishing a policy.

## Public API version retirement

The agreed initial notice is at least 180 days before withdrawing a supported
public major version, once replacement and migration guidance are available.
This is an organisation-wide API compatibility policy, not an office override.
Configure future notice periods explicitly and honour already published support
dates. Public version compatibility is independent of UI releases.

## REST admission and retry policy

Configure rate/burst limits for standard and expensive
request classes, with one token per admitted attempt, including replays.
Authenticated balances follow the person/application across sessions and
clients. Bound inputs and worker execution separately; establish numerical
limits through representative load validation.

No numerical initial rate/burst or retry-retention value is adopted yet.
The [request contract](research/api-surfaces/request-behaviour.md) records
the required request behaviour and remaining retention/expiry decisions.

## Outage protection

During a confirmed application outage preventing desk check-in or acceptance of
waitlist offers, pause the affected no-show releases and offer expiries. Resume
with a configurable recovery grace period. No numerical initial grace value has
been agreed; configure it explicitly before enabling automatic expiry. These
protections do not extend a booking beyond its actual usable period.

## Agreed allocation rules

Among plans accommodating the same number of people, prioritise explicit
collaboration, then general team proximity, then previously unfulfilled demand.
Ordinary direct desk booking opens after the date's main allocation run; before
then, requests enter intent collection. Existing waitlist priority still applies
when capacity becomes available.
