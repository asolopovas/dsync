package main

import (
	"testing"
)

func TestReverseReplacementOrder(t *testing.T) {
	// User's config scenario
	replacements := []DBReplace{
		{From: "avianese.com", To: "example.test"},
		{From: "https://example.test", To: "http://example.test"},
	}

	// Forward Sync (Remote -> Local)
	// Expected: https://avianese.com -> http://example.test
	inputForward := "Check https://avianese.com now"
	expectedForward := "Check http://example.test now"

	// Current logic for forward
	resForward := ApplyDBReplacements(inputForward, replacements)
	if resForward != expectedForward {
		t.Errorf("Forward sync failed. Got: %s, Want: %s", resForward, expectedForward)
	}

	// Reverse Sync (Local -> Remote)
	// Expected: http://example.test -> https://avianese.com
	inputReverse := "Check http://example.test now"
	expectedReverse := "Check https://avianese.com now"

	// Current logic for reverse (as implemented in db.go currently)
	var reversedReplacements []DBReplace
	for _, r := range replacements {
		reversedReplacements = append(reversedReplacements, DBReplace{From: r.To, To: r.From})
	}

	resReverse := ApplyDBReplacements(inputReverse, reversedReplacements)

	// This is expected to FAIL with current implementation
	if resReverse != expectedReverse {
		t.Logf("Current Reverse sync failed as expected. Got: %s, Want: %s", resReverse, expectedReverse)
	} else {
		t.Errorf("Current Reverse sync passed unexpectedly? Logic might be already correct?")
	}

	// Proposed logic: Reverse the order of replacements
	var reversedOrderReplacements []DBReplace
	for i := len(replacements) - 1; i >= 0; i-- {
		r := replacements[i]
		reversedOrderReplacements = append(reversedOrderReplacements, DBReplace{From: r.To, To: r.From})
	}

	resReverseOrder := ApplyDBReplacements(inputReverse, reversedOrderReplacements)
	if resReverseOrder != expectedReverse {
		t.Errorf("Proposed Reverse sync failed. Got: %s, Want: %s", resReverseOrder, expectedReverse)
	}
}
