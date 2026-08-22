package services

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestPrimitiveRegexEscapesUserInput(t *testing.T) {
	pattern := primitiveRegex("file.+(pdf)")
	if pattern.Pattern != `file\.\+\(pdf\)` {
		t.Fatalf("pattern = %q, want escaped literal", pattern.Pattern)
	}
	if pattern.Options != "i" {
		t.Fatalf("options = %q, want i", pattern.Options)
	}
}

func TestMongoPipelineUsesDownloadsSortAndPagination(t *testing.T) {
	pipeline := mongoPipeline(bson.M{}, "downloads", -1, 16, 8)
	facet, ok := pipeline[3].(bson.M)["$facet"].(bson.M)
	if !ok {
		t.Fatal("expected facet stage")
	}
	data, ok := facet["data"].(bson.A)
	if !ok || len(data) != 4 {
		t.Fatalf("unexpected data pipeline: %#v", facet["data"])
	}
	sortStage, ok := data[0].(bson.M)["$sort"].(bson.D)
	if !ok || sortStage[0].Key != "downloads" || sortStage[0].Value != -1 {
		t.Fatalf("unexpected sort stage: %#v", data[0])
	}
	if data[1].(bson.M)["$skip"] != 16 || data[2].(bson.M)["$limit"] != 8 {
		t.Fatalf("unexpected pagination stages: %#v", data[1:3])
	}
}
