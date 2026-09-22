# IAM reliable messaging handoff candidate

M3 remains a draft. This procedure is review material and has not been executed in production. The SDK switch stays disabled until the final reviewer approves concrete rollout evidence. The rollback target is the reviewed M3-compatible image with SDK mode disabled and schema 38 retained; do not assume a pre-M3 binary can start against a later migration journal.

## Writer inventory

| Entry | Current behavior | Handoff treatment |
| --- | --- | --- |
| API authz UoW | Selects original or reliable Stager with the configured Relay mode | Freeze business writes during the handoff and verify the selected image/configuration |
| API legacy Relay | Claims and writes by event ID, bypassing SDK fencing | Every old process must terminate before SDK claiming; configuration in one process proves no cross-host exclusion |
| SDK Relay | Token/version/lease writes; admitted operations drain on Stop | Stop and verify process termination before legacy rollback; nonzero exit or remaining publishing rows require recovery |
| Fresh bootstrap stages 33 and 35 | Original Stager before schema 38 exists | Retain intentionally; only fresh-database initialization before runtime, no Relay claiming here |
| condition-authz-retire | Original insert-only Stager | Include in the maintenance freeze and review its transaction/clock; no claim/mark ownership |
| statistics-operations | Original insert-only Stager | Same maintenance boundary |
| scope-migrate | Original insert-only Stager | Same maintenance boundary |
| role-model-migrate | Original insert-only Stager | Same maintenance boundary |
| reviewer-scope | Original insert-only Stager | Same maintenance boundary |

The five maintenance commands create fresh version events in their caller transaction. They do not execute ClaimDue/MarkPublished/MarkFailed and must not be confused with the unsafe legacy Relay. Keeping these entry points temporarily does not prove all IAM MQ paths migrated; their retained scope and eventual Stage selection remain M3-06 review items.

## Read-only database check

Use the existing IAM_APISERVER_MYSQL_* environment variables with an appropriate read-only database account. Do not put credentials in command arguments. The command emits counts and booleans, never payloads, identities or tokens.

```sh
iam-maintenance reliable-messaging preflight --event-catalog=/app/configs/events.yaml --target=sdk --max-rows=10000 --timeout=30s
iam-maintenance reliable-messaging preflight --event-catalog=/app/configs/events.yaml --target=legacy --max-rows=10000 --timeout=30s
```

A report requires clean schema 38 or later. Raise the bounded budget deliberately when truncated; neither truncation nor a failed query passes. Exit 0 checks only the selected database condition. Always retain the report with its database timestamp, image/version evidence and the separately verified process state, then rerun immediately before handoff. No report grants rollout authorization.

## Forward handoff

1. Record exact old/candidate image revisions, complete producer/Relay/subscriber topology, rollback-compatible image and approved observation/latency/retention thresholds. Verify backup/recovery procedure and migration 38 in isolation first.
2. Audit original scheduling clocks against the agreed fixed UTC+8 business convention. API currently uses loc=Local; a read-only observation on serverB returned +0800 for iam-apiserver at M2 image d8973629. This alone does not establish every historical writer's clock. Released maintenance helper connections parse dates as UTC; the candidate explicitly selects UTC+8 for parsing and MySQL session time. Maintenance Stage writes are immediate, whereas failed/publishing scheduling was written by the API Relay. Check old pending/failed/publishing samples and deployment history before cutover. The adapter must not silently reinterpret or bulk rewrite old times. Verify MySQL server/session settings independently of container timezone.
3. Freeze authorization and maintenance writes for the window. Stop all legacy Relay processes and verify they are terminal, including old deployment replicas. A cancel-only hook, empty lease scan or process configuration flag is insufficient. Termination may leave uncertain sends, which remain recoverable original rows.
4. Apply the approved additive schema migration separately. Capture preflight counts; classify unknown/quarantined/conflicting rows without deleting them. Preserve published rows as transport confirmations, not consumer completion. Valid unfinished publishing rows require SDK recovery after old-owner exclusion.
5. Start the reviewed candidate with enabled=true, the fixed UTC+8 business-clock policy and reviewed runtime bounds. Verify the API runtime timezone is UTC+8 and there is no legacy claimant elsewhere. Observe original IDs/payloads, delayed/stale recovery, quarantine and the full subscriber version graph. SDK lease arithmetic remains UTC in its dedicated column.
6. Execute the real authorization/decision acceptance cases and meet the measured recovery and observation thresholds. Failed convergence cannot be replaced by a healthy process or a published Outbox row. Release the write freeze only according to the reviewed rollout sequence.

## Application rollback

Freeze writes and stop SDK admission. Successful Stop joins admitted writes and drains underlying publisher calls before resource closure. The configured failure path logs and terminates nonzero; it does not claim drain success. If any publishing row or unresolved send remains, run a controlled SDK recovery phase under exclusive ownership before proceeding. Do not clear tokens, reset every published row or force rows into pending merely to pass the check.

Require a fresh target=legacy report with no publishing/quarantine/unknown/inconsistent/unfinished-conflict blockers, verified SDK process termination and the same original scheduling-clock convention. Use the reviewed schema-38-compatible image in disabled mode. Keep added columns and digests. Recheck actual policy convergence and decisions after rollback. Neither the old M2 image nor full database restoration is automatically proven by this procedure.

Schema down is a different operation: target=schema-down only checks current metadata non-use, matching the guarded down migration. It is not an application rollback shortcut. Never force a dirty migration journal clean to bypass a refused down.

## Remaining acceptance evidence

The isolated proofs cover original transactions, raw NSQ delivery, fencing, compatible scheduling and sequential Store handoff. They do not yet prove two real service-process forward/backward transitions, all replica termination, production load/locks, all subscribers or real authorization decisions. Those evidence gaps, numeric thresholds, ownership and private-repository CI access remain required before M3 approval.
