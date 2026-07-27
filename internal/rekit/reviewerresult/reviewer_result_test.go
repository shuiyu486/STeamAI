package reviewerresult

import (
	"strings"
	"testing"
)

func TestDecodeRejectsMalformedReviewerResultObject(t *testing.T) {
	_, err := Decode([]byte(`{"packetId":"packet-1","routeId":"route-1"}`))
	if err == nil || !strings.Contains(err.Error(), `reviewer result missing required field "shardId"`) {
		t.Fatalf("Decode malformed reviewer result error = %v", err)
	}
}

func TestDecodeAcceptsReviewerResultContract(t *testing.T) {
	result, err := Decode([]byte(`{
		"packetId":"packet-1",
		"routeId":"route-1",
		"shardId":"shard-01",
		"items":["item-a"],
		"reviewerSession":"reviewer-session-1",
		"decision":"accept",
		"confidence":"high",
		"summary":" accepted ",
		"evidenceRefs":[],
		"risks":[],
		"conflicts":[],
		"recommendedVerdict":"accepted",
		"routeOutput":{}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary != "accepted" || result.PacketID != "packet-1" || result.ShardID != "shard-01" {
		t.Fatalf("Decode did not normalize reviewer result: %+v", result)
	}
}
