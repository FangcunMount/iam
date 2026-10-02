# Historical SMS wire

sms-v061-wire.json is captured by scripts/testing/capture-legacy-sms-wire.sh from fixed IAM source a929f5301ed8265a8f44726b474da27ae10cb1c1 and its component-base v0.6.1 dependency. The script executes the original event payload and message encoder against synthetic OTP/phone values. Tests compare the complete emitted bytes; they do not regenerate expected bytes with the current SDK.

Only historical capture builds import the retired messaging package. The current test graph must not import it. The fixed original tag/source is retained for repeatable capture and rollback.
