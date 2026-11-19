# Use a domain-first modular monolith

Organise the retained Go application as a domain-first modular monolith with
published module interfaces and owned data, using ports and adapters inside
complex modules. This keeps consistency rules local and isolates external
systems, at the cost of enforcing module dependencies and maintaining explicit
cross-module workflow contracts. The [decision analysis](../research/architecture-options-spaceweave.md)
records the reasoning. The [full five-option review set](../research/architecture-options/README.md)
compares detailed repository layouts and tradeoffs; the final module map remains
subject to ownership and consistency validation.

One versioned application image supports independently scaled Kubernetes
deployments for HTTP, allocation, Exchange synchronisation, tracing and
maintenance roles.
