# Keep attendance planning inside Booking

Booking owns attendance intents, allocation cycles/groups, planning state and
capacity commitment, keeping demand changes and final validation under one owner.
Isolate the solver behind a private interface and calculate outside long
transactions; allocation workers retain independent scaling within the shared
release. This broadens Booking while removing a separate Planning module's
cross-owner demand validation and progress-coordination contracts.
