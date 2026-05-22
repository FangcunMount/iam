# Authn Authentication Domain

Authentication domain code owns proof validation, strategy selection, and the
`Principal` produced by a successful authentication decision. It does not issue
access or refresh tokens.

## Boundaries

- Domain owns login identity lookup, credential verification, proof shapes, and
  authentication decisions.
- Application owns sign-in method selection, proof construction, token lifecycle,
  and transport-facing orchestration.
- Infra owns persistence adapters, external identity providers, and token
  encoding implementations behind application ports.

## Core Concepts

- `AuthCredential` is a method-specific proof presented to a strategy.
- `AuthStrategy` validates one proof kind and returns an `AuthDecision`.
- `Principal` captures the authenticated subject, tenant context, realm, and
  optional claims for downstream session creation.

Password authentication resolves tenant scope from the matched login identity
(`ScopedTenantID`, numeric realm, or request tenant) and keeps `Principal.Realm`
aligned with the identity realm.

## Dependency Rules

- `domain/authn/authentication` may depend on login identity and credential domain
  packages plus shared error/meta helpers.
- Domain authentication must not import token codecs, JWKS, signing keys, or
  transport packages.
