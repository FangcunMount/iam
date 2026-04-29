package outbox

import sharedoutbox "github.com/FangcunMount/iam/pkg/outbox"

type PendingEvent = sharedoutbox.PendingEvent
type Store = sharedoutbox.Store
type StatusBucket = sharedoutbox.StatusBucket
type StatusSnapshot = sharedoutbox.StatusSnapshot
type StatusReader = sharedoutbox.StatusReader
