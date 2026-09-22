# M2 original IAM UoW proof

Run `scripts/testing/reliable-messaging-proof.sh /absolute/path/to/reliable-messaging` with a local Docker context. The script creates a temporary Go workspace and isolated MySQL from the SDK's pinned Compose definition. It never rewrites either go.mod, accepts no external DSN, and cleans its own containers/database. Missing dependencies fail; the required proof does not skip.

Tested against SDK commit 42fa4c3, IAM baseline 24dbe924, MySQL 8.0.44. `TestReliableMessagingOriginalUoW` invokes the actual authz UoW, role repository, policy-version repository and historical Outbox stager. A thin test-only bridge obtains the original transaction with RequireTx and calls SDK BindGORM. Role/version/legacy event/SDK comparison intent commit or roll back together, including an SDK identity-conflict error. The committed legacy raw payload remains exactly `{"version":2}` and SDK claim/confirmation retains the original identity/fingerprint.

The SDK standard table is a comparison fixture, not a proposed production dual-write/dual-publish scheme. The historical producer has no persisted occurred_at field; this proof maps its stored created_at once. Production identity/time/routing policy, historical schema extensions, old/new claimant exclusion, migrations, live consumers and cutover remain later gates. This proof does not replace old-table claim/mark algorithms or establish production acceptance.

Only a tagged test and isolated test runner are added. Production wiring, dependencies and configuration are unchanged. The SDK remains provisional and this draft should not be read as an IAM rollout approval.

## Original policy consumer with real ACK loss

`TestReliableMessagingPolicyConsumer` runs two real IAM policypublication handlers and immutable runtimes against MySQLSource, receiving raw version-only payloads from the SDK publisher over NSQ. Each test instance uses its own channel (broadcast); this does not test the production ephemeral-channel lifecycle.

A TCP proxy drops each consumer's first FIN while preserving normal client accounting. NSQ really redelivers the same NSQ message ID with an increased attempt count. The handlers do not reload an already-loaded version, old versions do not regress the snapshot, and an intentionally unpublished database version is recovered by the original Reconcile path. Invalid version payloads still return errors. Local proof passed against SDK d13ee10 / NSQ 1.3.0 / MySQL 8.0.44; both consumers and proxies drained.

The first ACK-loss fixture disabled automatic response in-process and left client in-flight accounting unresolved at shutdown; that failed run is not accepted evidence. The wire-level FIN-drop fixture replaces it. No production consumer implementation was modified.

## Historical schema and fencing feasibility

`TestReliableMessagingHistoricalFencing` uses the actual historical table and stager/UoW in real MySQL. It adds candidate nullable token/lease and defaulted claim version/count columns in the disposable database; the original stager continues to write. Conditional claim/confirmation preserves the original event identity, raw version-only body and the existing failed-publication counter. Forced lease expiry and ownership transfer reject an old token while accepting the replacement.

The negative test then calls the unchanged original MarkEventFailed. Its ID-only update succeeds and changes the newly published record back to failed. This proves additive fields alone do not make overlapping old/new writers safe. Stop and drain all old writers, account for in-flight rows and establish a single executor before enabling a new adapter; rollback requires the reverse handoff. Compatibility DDL is not an execution fence.

The inline SQL is a feasibility prototype, not a complete SDK Store, deployable migration or production index benchmark. rm_claim_count is a delivery-claim counter, whereas old attempt_count counts failed publications; do not silently equate the two or reset a retry budget. The current original IAM Relay uses its configured fixed delay without an attempt-count limit. A new budget would be a separately reviewed behavior change.

Historical state mapping for the eventual host adapter:

| Stored state | Required treatment |
| --- | --- |
| pending | Preserve identity/payload/due time; claim only when due |
| failed | Preserve last error and failure count; retain old retry policy |
| publishing without token | Legacy in-flight work: resolve old executor ownership before reclaim; null token is not evidence of safe takeover |
| publishing with token | Require token, generation, status and unexpired lease for every write |
| published | Retain transport confirmation and skip automatic republish; it is not consumer completion |
| unknown/new isolation states | Keep visible and classify explicitly; no silent success, deletion or rollback to a legacy-invisible backlog |

The eventual adapter still needs this complete state mapping, corrupt/unknown record isolation, indexing and a reviewed forward/backward migration before M3 acceptance. The test proves a safe conditional-write mechanism and the unsafe overlap boundary; it does not approve production cutover.

## Host shutdown boundary characterization

`TestReliableMessagingShutdownJoinBoundary` runs the actual legacy `runOutboxRelay` and `runShutdownSequence` with a controlled admitted dispatch and database-close callback. The hook is supplied explicitly: cancel-only mirrors current `startRuntimeTasks`; cancel-and-join is a candidate contract, not a production change. The legacy case demonstrates database close while admitted dispatch remains active. The candidate waits for the Relay before database close. Both cases passed ten repetitions under the race detector; the proof script now runs this host-only test before the isolated storage/broker tests.

This is not a real SQL connection closure or NSQ durability test, and it does not assert that the current production dispatch always continues after cancellation. It establishes why cancel alone cannot satisfy the SDK drain contract. M3 must wire and test actual Run supervision, bounded shutdown, adapter drain and host resource ownership; those are not implemented by this characterization.

## M3 historical Store candidate

`TestReliableMessagingHistoricalStore` runs the new IAM `ReliableStore` against the exact historical 000006 DDL plus disposable candidate claim columns. Two concurrent claimers return one owner; retry preserves the original payload and increments only the legacy failure count, while claims use their own counter. Explicit SQL lease expiry allows a replacement; all three stale mutations are rejected. Unsupported rows become visible quarantine entries. Identity unit tests preserve raw extensions and stable historical creation time while rejecting unknown mappings.

This candidate is not wired into the service. Production migration/indexes, rollback handoff, immutable-content conflict persistence, original Stage integration and the full runtime lifecycle still require implementation and validation. Lease expiry here is injected, not a process-crash proof. The script copies the original schema into its isolated container and fails if it is missing. Database integration uses a non-race-instrumented executable; separate host package race checks do not change that distinction.

The candidate now pins `rm_fingerprint` on first historical claim and rejects a later valid-JSON content edit under the same identity into visible quarantine without deleting evidence. Historical rows with no prior digest cannot retroactively prove pre-migration integrity. New intents still need transactional digest installation in the forthcoming Stage adapter. Real MySQL proves post-claim tampering rejection; this does not complete same-ID append conflict handling.

## M3 original-transaction Stager candidate

`ReliableStager` now borrows the original MySQL transaction, writes the historical intent and digest together, and checks duplicate identity under the unique-index/row lock. An identical append preserves the entire previous row, including its persistence timestamp and delivery state. A conflicting append returns the SDK conflict error; the host must roll back. The actual authz UoW proof verifies policy version commit, duplicate-row preservation, role/version rollback on conflicting payload, and no retained intent after host abort. It uses the original 000006 schema; there is no SDK comparison table or production dual write in this test.

The full isolated proof script passed after fixing test-only queries to the actual `authz_roles` and `authz_policy_versions` tables. Production wiring, migration/indexing, exclusive cutover/rollback and lifecycle supervision remain open. Remote CI currently cannot download the new private SDK dependency without cross-repository read authorization; local success is not CI success.

## M3 migration 000038

Store/Stager proofs now install the actual 000038 additive migration instead of inline prototype DDL. A separate disposable database verifies old row/default preservation, removal and reapplication of unused schema, rejection of direct schema rollback after use, digest preservation and the expected indexes/token binary collation. Another empty database runs the complete real migration chain and bootstrap to 38. The script explicitly copies the original bootstrap SQL and event catalog needed by the compiled test binary; missing files fail rather than skip.

Schema rollback after SDK use is intentionally refused. Application rollback must retain these additive columns and perform an exclusive handoff after resolving publishing/quarantine/unknown records. Run preflight before invoking a migrator down step: a refused down follows the normal dirty-version procedure, and must not be forced clean without examining the actual schema. The tests execute the down SQL directly and do not claim this is a full application rollback rehearsal. Index existence is verified, not production query latency or lock duration.

## M3 supervised runtime candidate

`ReliableRuntime` explicitly starts one SDK Run supervisor, retries returned scan failures after a bounded host-configured delay, and prevents repeated Start calls. Stop closes admission, waits for admitted SDK publish/writeback completion, then drains the borrowed publisher. Join/drain cancellation returns an error; a subsequent Stop can finish after the host resolves the outstanding operation. It never closes DB or producer resources itself. Stop before Start permanently prevents admission.

The actual SDK Relay with controlled Store/Publisher doubles proves recovery after a failed scan, surviving writeback after admission cancellation, join before driver drain, observable drain cancellation and retry, and no further scans. Error and unexpected nil exits both support cancellation during restart delay. The messaging package passed ten race-instrumented repetitions. These checks are included in the proof script; they are host runtime evidence, not SQL/NSQ durability tests.

The candidate is not yet selected by platform/process bootstrap. Production still uses its original cancel-only relay hook. M3 must connect the runtime, implement resource-close behavior on drain failure, configure a host-owned producer and validate exclusive handoff before enabling it. An ordinary lifecycle hook that logs Stop errors and continues closing the DB does not satisfy this contract.
