# Separate workplace definitions from dated booking inventory

Workplace owns offices, buildings, floors, space definitions and baseline
neighbourhoods; Booking owns dated capacity and temporary neighbourhood
adjustments. Allocation plans against a versioned snapshot outside long database
transactions, then Booking revalidates and commits bounded batches. This keeps
ownership clear and preserves confirmed allocations while planning runs.
