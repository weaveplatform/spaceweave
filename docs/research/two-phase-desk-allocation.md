# Two-phase desk allocation

Primary sources reviewed on 2026-11-21. This note explains the accepted
[two-phase allocation decision](../adr/0037-form-collaborator-groups-before-packing-desk-allocations.md)
and compares implementation candidates. It selects no solver, foreign runtime
or repository architecture, and reports no executed benchmarks or tenant evidence.

## Decision and phase boundary

SpaceWeave is a Go platform allocating application-owned desks. Phase one forms
date-specific collaborator groups from attendance intents, including singletons.
Externally maintained teams inform relationships; overlapping team memberships
must not duplicate a person's demand. Phase two packs those groups into existing
seats, satisfying each member's mandatory desk features and required parking.
Member-to-desk matching establishes a group's feasible placement; it does not
replace the two phases with joint individual assignment.

The objective order is maximum accommodated demand, explicit collaboration,
general team proximity, then previously unfulfilled demand. Within proximity,
sitting together precedes minimum walking distance. Accommodated demand counts
people with valid required outcomes, rather than groups or partially satisfied
requirements. Preserve this ordering through a justified objective formulation;
arbitrary weights could let many small preferences outweigh accommodating someone.
Unequal placement quality is expected. Fairness, equal group sizes and partial
credit are not additional objectives. No arbitrary runtime budget weakens the
goal of the best allocation.

The phase boundary should carry dated membership, relationship provenance and
the versions used to form groups. Packing receives those groups plus the
effective neighbourhoods, existing desks, member eligibility/features, parking
capacity, protected bookings and spatial relationships. Group-count and size
rules, group indivisibility and the fallback for a group that cannot fit remain
unresolved. A library's default balance constraint cannot decide those policies;
automatic splitting or regrouping is not an accepted fallback.

## Technology candidates

| Candidate | Relevant capability | Application work and limits |
| --- | --- | --- |
| METIS for phase one | Multilevel graph partitioning supports weighted edges, multiple vertex-weight balancing constraints, target partition weights and imbalance tolerances. | Map dated people and collaboration/team relationships into a graph. Choosing the part count and any balance parameters requires application rules. The returned cut is not a desk-feasibility result. See the [manual, §§3.2–3.3 and 5.8](https://papers.karypis.org/glaros/files/sw/metis/manual.pdf) and [current partitioning entry point](https://raw.githubusercontent.com/KarypisLab/METIS/master/libmetis/kmetis.c). |
| KaHIP for phase one | Its graph-partitioning interface accepts vertex/edge weights, part count and imbalance and returns a partition and edge cut. | Evaluate its grouping quality under the same application rules as METIS. Its balance/cut model supplies neither mandatory feature matching nor required parking commitments. See the [maintainer's description and library example](https://github.com/KaHIP/KaHIP#linking-the-kahip-library). |
| Custom CP-SAT or mixed-integer programming (MIP) for phase one | Boolean/integer variables can express co-membership and a chosen collaboration objective. OR-Tools demonstrates constrained group assignment in [CP-SAT and MIP](https://developers.google.com/optimization/assignment/assignment_groups). | Design a grouping model after the missing grouping rules are settled; that example's predefined worker combinations are not SpaceWeave's grouping algorithm. Neither a generic model nor a partitioner establishes superior allocation quality without evaluation. |
| Custom CP-SAT or MIP for phase two | Assignment variables and capacity constraints can represent placement into existing inventory; both approaches are illustrated by the [OR-Tools assignment example](https://developers.google.com/optimization/assignment/assignment_example). | Add group placement, member-specific eligibility, linked parking, protected claims and the ordered objectives. Solver examples do not provide this complete model. |

## Feasibility and assurance

Graph cuts reward keeping relationship edges within a partition. Balancing vertex
weights can approximate headcount or aggregate resource demand. These are
different from proving that distinct compatible desks exist for all members.
For example, two desks may collectively have one feature-X and one feature-Y
occurrence, but both features may belong to the same desk. Two members separately
requiring X and Y cannot both use it. This constructed counterexample shows why
aggregate feature counts are insufficient; feasibility needs an actual matching
that also respects all concurrent groups. Team edges crossing a partition and
walking distances between physical seats are also different quantities.
The existing [floorgraph](../../internal/floorgraph/graph.go) models walls;
approved seating relationships and traversable connectivity require separate
validation before they can supply operational proximity scores.

Packing therefore needs group placement candidates with a member-to-seat witness,
or equivalent constraints tying each member assignment to its group's placement.
Exclude ineligible seats, prevent duplicate seat/person allocations, and account
for the existing inventory's time/date scope. Required parking participates in
feasibility and commitment: specific bays need exclusive assignment, while pooled
parking needs capacity accounting. A desk outcome dependent on parking cannot be
confirmed without it. These are application constraints, not optional penalties.

Generic bin packing is an incomplete analogy. Its conventional objective is to
use the fewest bins to hold every item; SpaceWeave has fixed existing inventory
and seeks the greatest accommodated demand before collaboration/proximity
preferences. The [OR-Tools packing guide](https://developers.google.com/optimization/pack/bin_packing)
explicitly distinguishes bin packing from selecting valuable items for fixed
capacity. Neither example models the complete desk problem.

An exact phase-two solve proves only the best placement allowed by its supplied
groups, inventory and model. A poor grouping can exclude a better overall plan;
an optimal graph cut does not prove the best two-phase allocation. Evaluation
must examine both grouping quality and the resulting packing outcomes. This
limitation identifies required evidence, without authorising an automatic
membership-changing recovery loop.

Record the objective vector, solver/model versions, parameters, status, incumbent
and objective bound. CP-SAT requires integer constraints and distinguishes feasible,
infeasible, invalid and unknown results in its [solver documentation](https://developers.google.com/optimization/cp/cp_solver).
Its [status definition](https://raw.githubusercontent.com/google/or-tools/stable/ortools/sat/cp_model.proto)
also permits `OPTIMAL` when a configured gap limit is met. The
[parameter source](https://raw.githubusercontent.com/google/or-tools/stable/ortools/sat/sat_parameters.proto)
explains that an absolute gap below one establishes optimality for an integer
objective. Inspect bounds, tolerances and objective scaling before claiming a
proof; a stopped feasible search alone cannot establish the best plan.

Booking retains planning ownership and a private solver boundary under
[ADR 0027](../adr/0027-keep-attendance-planning-inside-booking.md). A native Go
baseline implementing grouping and placement search remains possible, with its
quality and proof limits measured explicitly. METIS and KaHIP expose native
library interfaces; OR-Tools' [documented distributions](https://developers.google.com/optimization/install)
cover C++, Python, Java and C#. Protobuf Go package metadata is not evidence of
a supported Go solver binding. Foreign-library integration or a separate process
would need its own operational evaluation; none is approved by this comparison.

## Completed-plan publication

The publication unit is the completed plan for the relevant office and attendance
date; a cycle spanning several dates can contain several plans. This does not
prove that scopes with shared people, parking or other constraints can be solved
independently; their dependencies still require validation.

Persist a coherent input snapshot before solving outside database transactions.
Include grouping inputs as well as desk/parking facts, relevant policies and
previous unmet demand. PostgreSQL's default Read Committed isolation can expose
different committed states to successive queries; a short Repeatable Read
transaction is one snapshot-capture candidate, not a lock held throughout search.
See [PostgreSQL 18 transaction isolation](https://www.postgresql.org/docs/18/transaction-iso.html).

Publication must revalidate relevant versions, eligibility and capacity under
guards shared with competing writers. A grouping-input change can invalidate
phase one as well as packing. A final transaction's isolation does not itself
refresh inputs persisted earlier. Preserve previously published bookings and
required desk/parking consistency from
[ADR 0014](../adr/0014-use-explicit-allocation-cycles-and-atomic-group-placement.md),
while publishing the completed, validated plan together. Intermediate preparation
must not progressively confirm groups or expose a partial plan as completed.

The concrete publication mechanism remains open: a guarded transaction and
unpublished staging with guarded activation have different implementation costs.
Any candidate must enforce capacity exclusivity, atomic visibility, crash recovery
and retry-safe publication/outbox effects. A pointer flip alone cannot establish
those properties. In particular, an index over staged claims cannot use a partial
predicate consulting another table's published-plan flag: PostgreSQL limits such
predicates to the indexed table and immutable expressions, excluding subqueries.
See [PostgreSQL 18 CREATE INDEX](https://www.postgresql.org/docs/18/sql-createindex.html).

## Required validation

These cases are specified here, not executed:

- Exhaustively enumerate tiny grouping and packing cases under the agreed rules,
  checking objective order and cases where grouping restricts the best packing.
- Exercise overlapping teams without duplicate demand and singleton groups.
- Reproduce the feature-distribution conflict above and scarce required parking;
  verify every published member has an eligible seat and required parking.
- Change grouping inputs during search and race publication with protected claims;
  reject stale results without altering previously published bookings.
- Crash before, during and after publication, then retry; readers must see either
  unpublished work or the completed valid plan, with consistent capacity and
  retry-safe follow-up effects.

Engine selection, grouping rules, unmet-group handling, precise proximity scoring
and the publication mechanism need review and evidence. Source review establishes
capabilities and constraints; it establishes no workload performance or allocation
quality result.
