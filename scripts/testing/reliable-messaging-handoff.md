# IAM standard Outbox handoff candidate

The user selected a standard table after legacy drain. M3 is a draft: no production cutover or data cleanup has been performed. Old records remain evidence. Do not enable SDK mode solely because a data preflight exits zero.

## Writer inventory and ownership

| Entry | Candidate behavior | Remaining handoff requirement |
| --- | --- | --- |
| API authz UoW | StandardStager in SDK mode; original Stager in legacy mode | Freeze writes and verify all instances use the reviewed image/mode |
| API legacy Relay | Original ID-only claims/writeback in legacy mode | Finish legacy work and terminate all old owners before SDK cutover |
| SDK Relay | Shared Store with token/version/lease conditions | Stop admission and join/drain before releasing resources; retain recovery if standard work remains |
| Fresh bootstrap 33/35 | Original insert-only Stager before standard migration 38 | Retained one-time legacy bootstrap; actual drain and sequential messaging-process handoff proven in isolation, complete-service rehearsal still required |
| condition-authz-retire, statistics-operations, scope-migrate, role-model-migrate, reviewer-scope | Explicit --outbox-mode after standard schema installation; shared maintenance selector | Match the reviewed live Relay and freeze during handoff; complete business/cutover acceptance |

Maintenance writers use `NewMaintenanceStager` before Apply/Rollback. On clean pre-38 databases without a standard table, omitted mode preserves the old writer. After standard schema installation an explicit `--outbox-mode=standard` or `--outbox-mode=legacy` is required. A dirty/missing journal is rejected; migrated databases must retain the standard schema in either mode. Standard mode rejects unfinished legacy work; legacy mode rejects unfinished standard work. Status/preflight/verify do not require this choice and retain their existing read-only paths. Existing fingerprint, actor, freeze and private-report requirements remain.

The role migration wrapper forwards `IAM_ROLE_MODEL_OUTBOX_MODE` to its apply command. The reviewer provisioning script forwards `--outbox-mode` in its existing scope helper (its current top-level CLI does not expose scope operations; no new operation has been enabled). Direct reviewer-scope uses the same Go selector. Do not assume that an older copied binary/script includes these protections; inventory its reviewed version during handoff.

The mode must match the running recovery owner. Empty tables cannot establish that mode or stop an old process from racing the check. This is an explicit operator choice and a data gate, not an ownership lock. All API and maintenance writers must be frozen during handoff.

## Data checks

Using existing IAM_APISERVER_MYSQL_* settings, a reviewed database account and the compiled candidate:

```sh
iam-maintenance reliable-messaging preflight --event-catalog=/app/configs/events.yaml --target=sdk --max-rows=10000 --timeout=30s
iam-maintenance reliable-messaging preflight --event-catalog=/app/configs/events.yaml --target=legacy --max-rows=10000 --timeout=30s
```

The report requires clean schema 38 and sufficient combined row budget. sdk requires no unfinished legacy work; legacy requires no unfinished standard work. Unknown, quarantined or malformed work requires classification. Published content anomalies stay reported, without becoming retry candidates. schema-down additionally requires zero standard rows, including published evidence. Neither writer exclusion nor production authorization is inferred.

Record image/revision, report database_time (+08:00), process state and consumer versions together. Repeat immediately before handoff. Bounded snapshots cannot prevent another process from writing after the snapshot.

## Fresh database sequence

Migrations 33/35 require the original notification transaction before table 38 exists. Keep the freshly initialized service in reviewed legacy mode under a write freeze, drain those original notifications through its actual Relay, verify subscriber convergence, terminate the legacy owner, then run SDK preflight and enable the standard mode. Do not rewrite old migrations, suppress the notifications, fake published states, or choose SDK mode merely from schema presence. Direct SDK-first initialization is not supported by this candidate. Retirement of this one-time legacy purpose belongs to the later legacy-removal design.

The isolated proof delivers both original notifications and a later standard message through NSQ using sequential child processes. It also checks a clean restart and a drained return to legacy mode. Those children use actual messaging composition but are not the full API service; the production signal, topology, writer-freeze and business checks remain mandatory.

## Forward transition

1. Review exact old/new images, fixed SDK revision, complete writer/subscriber topology, freeze window, recovery/latency thresholds and retained rollback resources.
2. Audit MySQL server/session/driver clocks independently. Business clock is UTC+8; standard scheduling/lease columns use the documented UTC contract. Do not shift historical DATETIME digits in bulk.
3. Verify standard migration 38, full bootstrap and application rollback in isolation. The migration adds rm_outbox without editing historical data. Only an empty standard table can be removed.
4. Freeze authorization changes and relevant maintenance writes. Let the legacy chain handle pending/failed work and classify uncertain publishing. Keep original identities. Confirm target policy instances converge; old-table unfinished count zero alone is insufficient.
5. Stop all legacy owners and verify termination and known in-flight outcomes, including old replicas. Apply the reviewed migration and obtain a fresh sdk data report. No old process may continue writing the historical table after this point.
6. Start the reviewed candidate in SDK mode, verify standard writes/claims/metrics and no stranded legacy work. Readiness reports legacy arrivals, quarantine and unknown states. Run real authorization propagation and decision cases before releasing the freeze according to the reviewed sequence.
7. Retain old table and images through the agreed observation window. Archive or removal is a separate reviewed operation.

## Backward transition

The preferred application rollback keeps standard schema and its recovery owner. If returning to legacy Outbox mode, freeze all writes, settle standard unfinished records with the SDK, stop/join/drain SDK execution, verify process termination, then require a fresh legacy data report. A failed drain is a nonzero process exit with remaining recovery responsibility, not graceful success.

Do not disable SDK recovery while new-table work remains; startup checks reject that combination. Do not dual-write, copy every published row, clear tokens or fake completion to pass the gate. A compatible candidate image with retained schema is the rollback target; an old M2 binary is not automatically proven to accept migration 38.

## Required remaining evidence

Actual platform construction and sequential data-state tests do not establish multi-process exclusion, all writers retired, full rollback, all consumer instances, business decisions or measured production thresholds. Those gates and private SDK CI access remain required before final review. qs-ai execution/recovery files are outside this change.
