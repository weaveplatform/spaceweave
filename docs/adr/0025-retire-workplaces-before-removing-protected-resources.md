# Retire workplaces before removing protected resources

Retirement can be immediate or scheduled at an office-local date/time; publishing
it prevents new bookings for use on/after that point while preserving existing
confirmed commitments and their normal management workflows.
Pre-existing unconfirmed holds/offers revalidate at confirmation and cannot
confirm excluded use; late Exchange effects require reconciliation.
Deletion is separate and reports a dependency conflict until blocking bookings,
children or other references are resolved; it never silently retires resources,
cancels bookings or cascades through the hierarchy. This keeps UI and automation
removal predictable at the cost of a deliberate retirement/resolution workflow.
Authorised reactivation revalidates definitions, calendars and dated inventory
while retaining the same identity, commitments and history.
