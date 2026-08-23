package factory

import (
	"testing"

	"go.mongodb.org/mongo-driver/mongo"
)

func TestDuplicateKeyLeaseErrorIsRecognized(t *testing.T) {
	err := mongo.WriteException{WriteErrors: []mongo.WriteError{{Code: 11000}}}
	if !mongo.IsDuplicateKeyError(err) {
		t.Fatal("expected duplicate-key lease error to be recognized")
	}
}
