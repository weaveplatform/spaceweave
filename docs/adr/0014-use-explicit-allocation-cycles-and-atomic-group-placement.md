# Use explicit allocation cycles and atomic group placement

Each office defines allocation cycles with attendance dates, an intent cutoff
and processing time, handling late changes incrementally. Commit each group
placement atomically with its required linked parking, and replan affected
uncommitted groups when capacity changes. This coordinates the cohort while
preserving completed allocations.

Ordinary direct desk booking opens after the date's main allocation run. Before
then, requests enter intent collection so the cohort can be planned together.
