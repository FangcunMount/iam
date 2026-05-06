# Authn Application Layer

Authn application code owns caller-facing use cases. It coordinates domain
authentication, account onboarding, token lifecycle, and JWKS publication without
owning JWT encoding or signing-key lifecycle behavior.

## Boundaries

- Domain owns account, credential, session, authentication proof, and
  authentication decision rules.
- Application owns sign-in method selection, proof construction, internal bearer
  reauthentication, token lifecycle orchestration, onboarding, account
  administration, and JWKS public/admin use cases.
- Infra owns JWT/JWS compact encoding, claims mapping, signing-key lifecycle,
  Redis stores, MySQL repositories, and external provider adapters.
- Transport owns REST/gRPC binding, DTO conversion, and error mapping.

`jwt_token` is an internal bearer reauthentication method. It is understood by
`application/authn/login` only; it is not exposed through the public REST/gRPC
login contract and is not a domain authentication strategy.

## Use Cases

### AccountOnboarder

Creates or reuses the identity user, guarantees the self profile link invariant, and
creates or reuses the Authn account and credential in one application flow.

### LoginApplicationService

Selects the sign-in method from the request, constructs a method-specific proof
or bearer verification request, calls the domain authenticator when appropriate,
and asks the token use case to create the session token pair.

Supported public method kinds are password, phone OTP, WeChat mini program, and
WeCom. Bearer-token reauthentication remains an internal application concern.

### TokenApplicationService

Provides the transport-facing token lifecycle facade:

- issue service token
- refresh token pair
- revoke access token
- revoke refresh token
- verify access token

JWT is only one infrastructure implementation of the access-token codec behind
these application ports.

### JWKS Use Cases

JWKS application services expose public publishing and admin commands as DTOs and
ports. Signing-key state transitions and key material handling live in
`infra/token/keyset`.

### Account Administration

Account application roles are split by caller intent: directory queries, profile
editing, status changes, and account session revocation side effects. The
aggregate interface remains available for existing container capability wiring.

## Dependency Rules

- `application/authn` may depend on Authn domain packages and application ports.
- `application/authn` must not import JWT libraries or `infra/token`.
- Signing-key lifecycle behavior must stay in `infra/token/keyset`.
- Domain Authn packages must not mention JWT, JWKS, bearer-token reauthentication,
  key ids, algorithms, or token encoding details.
