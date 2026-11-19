# Keep app-managed delegation local

App-managed delegation authorises app-mediated bookings, including rooms, while
leaving native Outlook delegate permissions unchanged. The resulting room
bookings, edits and cancellations still synchronise bidirectionally with Exchange
Online. This gives users booking delegation within SpaceWeave while native
calendar access retains its existing authority.

Exchange-derived grants initially map explicit calendar-delegate relationships;
other permissions require deliberate mappings retaining source, owner and
authorised actions. Exchange room policy and approved integration permissions
continue to apply.

Revoking an Exchange-derived grant removes that source of authority. A separately
approved app-managed grant remains governed by its own lifecycle.

For new actions relying on Exchange-derived authority, require verification
within a configurable fifteen-minute maximum age and refresh stale authority
before authorising. Check app-managed grants against current application state.
