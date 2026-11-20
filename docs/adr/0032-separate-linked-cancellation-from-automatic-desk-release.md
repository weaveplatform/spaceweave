# Separate linked cancellation from automatic desk release

Before either booking starts, explicit cancellation of either part of a
both-required desk/parking pair cancels both atomically; optional links permit
independent cancellation.
Automatic desk no-show or vacancy release leaves parking booked until its own
end or explicit release, accepting separate resource lifecycles because desk-use
evidence cannot establish that a car has left.
