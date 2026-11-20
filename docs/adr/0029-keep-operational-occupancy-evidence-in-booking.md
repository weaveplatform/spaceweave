# Keep operational occupancy evidence in Booking

Booking validates, deduplicates and owns the operational occupancy evidence used
for desk-release decisions, committing the resulting facts with its transitions.
Attendance owns reporting projections built from committed facts, with separately
configured reporting retention; this keeps release authority with the claim
owner at the cost of maintaining a derived reporting view.
