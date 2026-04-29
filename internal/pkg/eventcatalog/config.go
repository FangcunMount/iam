package eventcatalog

import (
	sharedcatalog "github.com/FangcunMount/iam/pkg/eventcatalog"
)

const (
	AuthzVersionChanged = "iam.authz.version_changed"
	LoginOTPSMS         = "iam.login_otp_sms"
)

type DeliveryClass = sharedcatalog.DeliveryClass

const (
	DeliveryClassBestEffort    = sharedcatalog.DeliveryClassBestEffort
	DeliveryClassDurableOutbox = sharedcatalog.DeliveryClassDurableOutbox
)

type Config = sharedcatalog.Config
type TopicConfig = sharedcatalog.TopicConfig
type EventConfig = sharedcatalog.EventConfig
type ValidateOptions = sharedcatalog.ValidateOptions

var Load = sharedcatalog.Load
var LoadWithOptions = sharedcatalog.LoadWithOptions
var Parse = sharedcatalog.Parse
var ParseWithOptions = sharedcatalog.ParseWithOptions
