package options

import (
	"encoding/json"
	"testing"

	"github.com/FangcunMount/reliable-messaging/transport"
)

func TestReadableManifestAndTransactionRoutesPreserveOwnerAndRevision(t *testing.T) {
	const topic = "iam.authz.version.v2"
	physical, err := transport.PhysicalTopicV3(topic, "TRANSACTION", "iam.mysql", "m3-1")
	if err != nil {
		t.Fatal(err)
	}
	row := transport.RouteBinding{Topic: topic, PhysicalTopic: physical, MessageType: "TRANSACTION", Owner: "iam.mysql", RouteRevision: "m3-1", NamingScheme: transport.ReadableNamingV1}
	old := row
	old.NamingScheme = ""
	old.PhysicalTopic, _ = transport.PhysicalTopicV2(topic, "TRANSACTION", "iam.mysql", "m3-1")
	raw, _ := json.Marshal(transport.RouteManifest{Version: 3, Routes: []transport.RouteBinding{row}, HistoricalRoutes: []transport.RouteBinding{old}})
	active, err := decodeRouteManifest(raw)
	if err != nil || len(active) != 1 || active[0] != row {
		t.Fatal("history selected as active", err)
	}
	options := TransactionOptions{StoreID: "primary", WriterEpoch: "m3-1", RouteRevision: "m3-1"}
	selected, err := transactionRoutes(active, "iam.mysql", options, []string{topic})
	if err != nil || len(selected) != 1 || selected[0].NamingScheme != transport.ReadableNamingV1 || selected[0].Revision != "m3-1" {
		t.Fatal("typed readable route lost", err)
	}
	row.NamingScheme = ""
	raw, _ = json.Marshal(transport.RouteManifest{Version: 3, Routes: []transport.RouteBinding{row}})
	if _, err = decodeRouteManifest(raw); err == nil {
		t.Fatal("readable scheme omitted")
	}
}
