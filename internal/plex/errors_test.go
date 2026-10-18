package plex

import (
	"strings"
	"testing"
)

type codedTestError struct{}

func (codedTestError) Error() string { return "private database data" }
func (codedTestError) Code() int     { return 6410 }
func TestDatabaseFailureSummaryDoesNotLeakData(t *testing.T) {
	s := SafeMessage(codedTestError{})
	if !strings.Contains(s, "6410") || strings.Contains(s, "private database data") {
		t.Fatal("expected safe diagnostic code only")
	}
}
