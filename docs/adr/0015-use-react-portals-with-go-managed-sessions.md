# Use React portals with Go-managed sessions

Use separate React/TypeScript staff/mobile-web and admin entrypoints, sharing UI
components and retaining the floor-plan renderer behind an interface. Go owns
authentication sessions and provider tokens; browsers use secure HttpOnly session
cookies. Backend authorisation and CSRF controls keep authentication and access
enforcement consistent across both portals.
