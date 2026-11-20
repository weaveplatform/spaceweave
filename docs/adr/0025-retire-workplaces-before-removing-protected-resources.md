# Retire workplaces before removing protected resources

An explicit retirement stops new bookings for an office, building, floor or space while
preserving existing confirmed commitments and their normal management workflows.
Deletion is separate and reports a dependency conflict until blocking bookings,
children or other references are resolved; it never silently retires resources,
cancels bookings or cascades through the hierarchy. This keeps UI and automation
removal predictable at the cost of a deliberate retirement/resolution workflow.
