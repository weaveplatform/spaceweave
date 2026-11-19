# Version runtime policies separately from deployment configuration

Store operational policies as versioned database records editable through
administration, separately from deployment configuration. Apply policy changes
to new cycles/bookings by default and retain existing confirmations' versions
unless explicitly migrated. This supports controlled runtime changes and
preserves the rules under which existing bookings were confirmed.

All initial values in [the policy defaults](../policy-defaults.md) are
configurable; validate values and cross-field consistency before publication.
