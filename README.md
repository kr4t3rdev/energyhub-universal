# EnergyHub Universal

Bootstrap 00: foundations for web, Go API, iOS and Android. No hardware integration or billing. Health is process liveness, not database readiness. Hardware compatibility is unverified.

The Go backend has an OIDC verifier connected to one protected route (`GET /v1/organizations`) and a PostgreSQL tenant-isolation boundary with two credentials: a read-only runtime role for the API and an administrative role that can only execute the membership functions. Membership administration is CLI-only (`cmd/membership`); there is no HTTP admin surface, and no telemetry, devices or real tenant data.

To bring up the local development stack (PostgreSQL roles, migrations, development OIDC issuer):

```sh
cd backend && bash scripts/dev-stack.sh
```

See [environment and evidence](docs/environment.md), [backend architecture](docs/architecture/backend-next-phase.md) and [OpenAPI](contracts/openapi/health.yaml). Run commands from repository root. Dependencies and builds may require network and platform SDKs.
