# M2 original IAM UoW proof

Run `scripts/testing/reliable-messaging-proof.sh /absolute/path/to/reliable-messaging` with a local Docker context. The script creates a temporary Go workspace and isolated MySQL from the SDK's pinned Compose definition. It never rewrites either go.mod, accepts no external DSN, and cleans its own containers/database. Missing dependencies fail; the required proof does not skip.

Tested against SDK commit 42fa4c3, IAM baseline 24dbe924, MySQL 8.0.44. `TestReliableMessagingOriginalUoW` invokes the actual authz UoW, role repository, policy-version repository and historical Outbox stager. A thin test-only bridge obtains the original transaction with RequireTx and calls SDK BindGORM. Role/version/legacy event/SDK comparison intent commit or roll back together, including an SDK identity-conflict error. The committed legacy raw payload remains exactly `{"version":2}` and SDK claim/confirmation retains the original identity/fingerprint.

The SDK standard table is a comparison fixture, not a proposed production dual-write/dual-publish scheme. The historical producer has no persisted occurred_at field; this proof maps its stored created_at once. Production identity/time/routing policy, historical schema extensions, old/new claimant exclusion, migrations, live consumers and cutover remain later gates. This proof does not replace old-table claim/mark algorithms or establish production acceptance.

Only a tagged test and isolated test runner are added. Production wiring, dependencies and configuration are unchanged. The SDK remains provisional and this draft should not be read as an IAM rollout approval.

## Original policy consumer with real ACK loss

`TestReliableMessagingPolicyConsumer` runs two real IAM policypublication handlers and immutable runtimes against MySQLSource, receiving raw version-only payloads from the SDK publisher over NSQ. Each test instance uses its own channel (broadcast); this does not test the production ephemeral-channel lifecycle.

A TCP proxy drops each consumer's first FIN while preserving normal client accounting. NSQ really redelivers the same NSQ message ID with an increased attempt count. The handlers do not reload an already-loaded version, old versions do not regress the snapshot, and an intentionally unpublished database version is recovered by the original Reconcile path. Invalid version payloads still return errors. Local proof passed against SDK d13ee10 / NSQ 1.3.0 / MySQL 8.0.44; both consumers and proxies drained.

The first ACK-loss fixture disabled automatic response in-process and left client in-flight accounting unresolved at shutdown; that failed run is not accepted evidence. The wire-level FIN-drop fixture replaces it. No production consumer implementation was modified.
