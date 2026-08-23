package db

import "testing"

func TestCloseWithoutConnectionsIsSafe(t *testing.T) {
	Close()
	Close()
}
