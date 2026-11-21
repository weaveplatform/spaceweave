# Separate workplace definitions from dated booking inventory

Workplace owns offices, buildings, floors, space definitions and baseline
neighbourhoods; Booking owns dated capacity and temporary neighbourhood
adjustments. Allocation plans against a versioned snapshot outside long database
transactions, then Booking revalidates the completed plan for publication;
bounded preparation work does not create progressive confirmations.
The [two-phase allocation decision](0037-form-collaborator-groups-before-packing-desk-allocations.md) defines that boundary.
This keeps ownership clear and preserves confirmed allocations while planning runs.
