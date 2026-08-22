package factory

import "testing"

func TestValidateMongoURI(t *testing.T) {
	valid := []string{"mongodb://localhost:27017", "mongodb+srv://user:pass@example.mongodb.net"}
	for _, uri := range valid {
		if err := validateMongoURI(uri); err != nil {
			t.Errorf("expected URI %q to be valid: %v", uri, err)
		}
	}
	invalid := []string{"", "postgres://localhost", "mongodb://"}
	for _, uri := range invalid {
		if err := validateMongoURI(uri); err == nil {
			t.Errorf("expected URI %q to be rejected", uri)
		}
	}
}
