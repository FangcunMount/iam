# IAM reliable messaging proofs

M3 uses the SDK standard `rm_outbox` table after the legacy path is drained. It remains a draft and is disabled in normal configuration. This document describes the current candidate, not production acceptance. Historical-table implementation/proof details remain in Git checkpoints d3036b14 and 4e20cf75; they are superseded as the M3 runtime route.

## Reproducible isolated proof

Run `bash scripts/testing/reliable-messaging-proof.sh /absolute/path/to/reliable-messaging` with a local Unix-socket Docker context. The script creates a unique MySQL/NSQ project and temporary build directory, and destroys only its own containers, network, volume and build directory. It never uses production connection settings.

IAM go.mod pins the UTC+8 correction c35167f as v0.1.0-m2.1.0.20260922073526-c35167f2ba5a (SDK draft #3). The script builds the declared module with GOWORK=off; the SDK checkout supplies isolated infrastructure fixtures only. This fixes the candidate dependency without implying that the SDK change has been approved, merged or tagged. Final release must select the reviewed version.

## Standard transaction and actual platform wiring

`StandardStager` maps the original policy event ID, actual occurrence time and payload bytes into Message, then calls SDK BindGORM/Append in the original IAM UoW. It never stages a second historical Outbox row. Role facts, PolicyVersion and standard Outbox commit together; host abort or identity-content conflict roll back together. Duplicate identity retains one row.

`TestReliableMessagingStandardUoW` exercises a real UTC+8 business connection, immediate claiming and absolute lease time. `TestReliableMessagingPlatformWiring` uses real platform construction, standard Stager/Store, host-owned Producer, supervised SDK Relay and NSQ delivery. Its EventBus is a placeholder because this test targets the dedicated publisher, not complete subscriber bootstrap. It verifies the original component-base envelope, application event ID, payload bytes and metadata, published state and no legacy-table insert.

`PolicyWirePublisher` is the IAM-owned transport codec. The standard intent retains the business payload and its fingerprint. At publication, an immutable transport copy carries the original component-base v0.6.3 envelope: UUID=event ID, original payload bytes, event_type, aggregate_type=PolicyVersion, aggregate_id=version and source=iam-outbox-relay. SDK transport remains broker-generic. Sending only raw payload would lose the application UUID/metadata even though the current consumer can decode raw JSON; that candidate wiring defect is corrected. Golden bytes cover the legacy encoding and payload whitespace; Unknown/Confirmed/Rejected outcomes and Drain are delegated without changing the stored intent.

SDK mode fails startup if migration 38 is missing/dirty or legacy unfinished work exists. Disabling SDK fails if standard unfinished work remains. The latter error cannot be suppressed by development degraded startup. These are data gates; no database query proves old processes have stopped or prevents an old writer racing the check. Cross-process handoff remains required.

The retained original consumer proof tests duplicate/old-version/ACK-loss/reconciliation behavior. It does not substitute for all deployed instances and real authorization decisions. M2 historical-fencing and UoW prototypes remain explicitly named baseline experiments, not production double-writing or historical adapters.

## Standard schema and rollback

The unshipped migration 38 now creates the SDK standard table in the host database. It leaves historical columns, rows and payloads untouched; IAM no longer adds legacy rm_claim fields. The real migration test verifies old-row preservation, an empty-table down/reapply, rejection of dropping a table even when it contains only published evidence, and identity/due/lease indexes. The full fresh migration/bootstrap test verifies the resulting table inventory.

Schema down is not application rollback. The guarded down SQL is tested directly; a failed migrator down may leave a dirty journal and must not be forced clean. Application rollback retains schema 38 and preserves standard pending/retry/publishing/quarantine ownership. See [handoff](reliable-messaging-handoff.md).

Fresh bootstrap retains the reviewed one-time legacy phase: migrations 33/35 commit their original notifications before standard schema 38 exists. The compiled preflight rejects immediate SDK cutover. This is an explicit controlled purpose for the retained legacy writer, not an SDK-first fresh startup or permission to discard notifications.

`TestMaintenanceBootstrapHandoff` starts four sequential OS child processes against the full migration fixture and actual NSQ. The first uses the original component-base EventBus and legacy platform Relay to deliver the two original notifications; only the actual Relay marks them published. The parent observes their original IDs, payloads and source metadata and verifies retained database records, then the real preflight accepts SDK data readiness. A second process uses SDK platform composition plus the original authz UoW to increment PolicyVersion and publish a standard message. A fresh SDK process does not reclaim that published row. After standard drain, a legacy rollback process commits/publishes one new original-table notification. The parent verifies both table counts, envelopes and data readiness again.

Each child must terminate successfully before the next starts, with SDK Stop/Drain before producer and DB close. This exercises messaging composition in real processes; it does not launch the complete API server, prove production writer exclusion, inject in-flight crash failures, or establish all consumer/authorization decisions. The existing full-chain migration fixture uses its UTC test connection; the child maintenance factory uses UTC+8. Separate timezone matrix tests remain the clock evidence, rather than inferring all production clocks from this handoff.

## Maintenance writer selection

Five mutation entry points share `NewMaintenanceStager`: condition-authz-retire, statistics-operations, scope-migrate, role-model-migrate and reviewer-scope. After standard schema installation they require explicit `--outbox-mode=standard|legacy` matching the reviewed live Relay. Pre-38 omission preserves the legacy writer; dirty/missing schema evidence and opposite-table unfinished records reject selection. Read-only operations retain their existing paths.

`TestReliableMessagingMaintenanceStager` uses real MySQL and the original host transaction. It checks implicit-mode rejection, pre-38 compatibility, dirty/missing schema, unfinished/unknown/case-malformed states in both directions, canceled queries, standard-only inserts and host rollback. Synthetic state fixtures establish data gates, not actual legacy drain or process exclusion. Individual maintenance business workflows retain their existing regression tests; this does not establish all five production workflows end to end.

## Read-only preflight and visibility

`iam-maintenance reliable-messaging preflight` reads both tables under a bounded read-only repeatable-read transaction. The combined row budget includes historical published rows. Truncation, query errors, malformed metadata, unknown state and invalid unfinished standard content cannot report successful SDK data readiness. Published anomalies are reported and never replayed. Output contains counts and an explicit +08:00 database timestamp, not identities, payloads or claim tokens.

Targets differ:

- sdk requires no legacy unfinished rows and a valid standard recovery state.
- legacy requires no unfinished standard rows; the legacy chain may still own known legacy pending/failed/publishing work.
- schema-down requires the standard table to be empty, including published records.

Every report leaves writer_exclusion_verified and cutover_authorized false. A clean report is neither a backup nor a process/consumer check. Production commands must use a reviewed account and scope.

The selected StandardStatusReader reports standard and legacy backlog independently and includes unknown states. Standard dates are decoded as UTC then displayed as UTC+8; legacy dates retain the audited UTC+8 business convention. Readiness rejects legacy work appearing after SDK cutover, quarantine and unknown standard states; ordinary backlog uses the existing configured age threshold. Runtime age thresholds are not yet measured production SLO acceptance.

## Clock and lifecycle contracts

Business/maintenance connection parsing and session time use UTC+8, while standard scheduling/lease storage has an explicit UTC contract. Real maintenance connection factories run inside a TZ=UTC container and must still select +08:00 sessions and preserve absolute instants. No existing timestamp is bulk rewritten.

ReliableRuntime has one serial supervisor. Stop cancels admission, joins admitted SDK publish/writeback, then drains borrowed transport operations; resources close afterward. Failed join/drain retains resources and propagates failure. The SDK never owns the DB or Producer. The real shutdown dispatcher child-process test requires nonzero failure exit; it is not a deployed SIGTERM/cutover test.

Candidate configuration under events.reliable_messaging: enabled=false, concurrency=1, lease=30s, publish_timeout=5s, write_timeout=5s, restart_delay=2s, shutdown_timeout=20s. The legacy_stale historical-adapter option is removed. Original outbox_relay_interval and retry delay still supply polling/retry policy. Unknown policy-notification sends retain original identity; this does not authorize replaying unknown model executions.

## Remaining acceptance

Reviewed fixed SDK dependency and cross-repository CI access; maintenance business acceptance and complete-service bootstrap/cutover rehearsal; real service-process forward/backward handoff; complete subscriber/decision acceptance; measured recovery/backlog/lock thresholds and observation window; final user review and separately authorized production rollout remain open. NSQ publish confirmation still does not prove consumer completion or durable replication.
