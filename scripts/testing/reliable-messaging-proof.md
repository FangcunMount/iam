# M2 original IAM UoW proof

Run `scripts/testing/reliable-messaging-proof.sh /absolute/path/to/reliable-messaging` with a local Docker context. The script creates a temporary Go workspace and isolated MySQL from the SDK's pinned Compose definition. It never rewrites either go.mod, accepts no external DSN, and cleans its own containers/database. Missing dependencies fail; the required proof does not skip.

Tested against SDK commit 42fa4c3, IAM baseline 24dbe924, MySQL 8.0.44. `TestReliableMessagingOriginalUoW` invokes the actual authz UoW, role repository, policy-version repository and historical Outbox stager. A thin test-only bridge obtains the original transaction with RequireTx and calls SDK BindGORM. Role/version/legacy event/SDK comparison intent commit or roll back together, including an SDK identity-conflict error. The committed legacy raw payload remains exactly `{"version":2}` and SDK claim/confirmation retains the original identity/fingerprint.

The SDK standard table is a comparison fixture, not a proposed production dual-write/dual-publish scheme. The historical producer has no persisted occurred_at field; this proof maps its stored created_at once. Production identity/time/routing policy, historical schema extensions, old/new claimant exclusion, migrations, live consumers and cutover remain later gates. This proof does not replace old-table claim/mark algorithms or establish production acceptance.

Only a tagged test and isolated test runner are added. Production wiring, dependencies and configuration are unchanged. The SDK remains provisional and this draft should not be read as an IAM rollout approval.
