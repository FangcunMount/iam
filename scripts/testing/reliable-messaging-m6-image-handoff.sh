#!/usr/bin/env bash
# The former image handoff requires the retired production NSQ/Outbox assembler.
printf '%s\n' 'IAM NSQ/Outbox image handoff is retired. Use the infra RocketMQ IAM original host transaction workflow with exact source SHAs.' >&2
exit 2
