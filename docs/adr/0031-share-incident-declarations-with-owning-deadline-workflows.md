# Share incident declarations with owning deadline workflows

Shared operational facilities own confirmed application-outage declarations,
including affected offices, actions and times; Booking and Events retain their
own protected deadline state and apply the relevant consequences, while Policy
owns configurable rules.
This avoids conflicting declarations across workflows at the cost of a shared
operational read/ordering contract; generic job execution and health observations
cannot independently decide business expiry.
