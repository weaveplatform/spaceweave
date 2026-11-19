# Require assignee check-in to mark a desk taken

System allocation assigns a desk to a staff member; the assignee's check-in marks
it taken. Anonymous occupancy remains separate evidence and does not retain an
unchecked-in allocation beyond its no-show grace period. This keeps a person's
desk claim attributable even when anonymous signals indicate physical occupancy.

After no-show release, return the desk directly to the pool even if occupancy is
still detected. This prevents unbooked physical occupancy from withholding
bookable capacity.

After check-in, signal loss preserves the taken desk until the booking ends, the
member releases it, or fresh vacancy evidence satisfies the configured release
rule.

Desk check-in is personal to the assigned staff member. Delegates can manage
bookings but do not check in for the member.
