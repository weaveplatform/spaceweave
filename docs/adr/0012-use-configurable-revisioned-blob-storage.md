# Use configurable revisioned blob storage

Store drawings and generated assets behind one blob interface with Azure Blob
Storage and Google Cloud Storage adapters, choosing the active provider through
deployment configuration. Publish immutable drawing/render revisions and keep
explicit current-version pointers and metadata in PostgreSQL. This preserves
historical plan context while keeping asset access portable across providers.

Recovery protection covers assets acknowledged as durable or published. Verify
their recovery copies before acknowledging durability; unfinished uploads and
processing remain retryable. Committed business records are also covered by the
agreed recovery targets.
