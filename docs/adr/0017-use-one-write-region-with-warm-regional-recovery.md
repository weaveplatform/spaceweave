# Use one write region with warm regional recovery

Run one active write region with multi-zone high availability and a
preprovisioned warm recovery region. This keeps booking capacity under one
write authority while targeting a five-minute regional RPO and fifteen-minute
RTO. Fence the previous writer before promotion and validate recovery across
the database, assets, identity, routing and workers together; asynchronous
replication alone does not guarantee those targets.

If regional recovery replication exceeds the five-minute loss budget while the
primary remains healthy, continue serving and raise a critical alert. Failover
that could lose more than the budget requires an explicit operator decision.
This preserves booking availability while explicitly accepting temporary
exposure beyond the RPO target.
