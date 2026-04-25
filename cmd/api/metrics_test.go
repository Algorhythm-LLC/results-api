package main

import "testing"

func TestOrderByForLeaderboardMetric(t *testing.T) {
	s, err := orderByForLeaderboardMetric("sharpe")
	if err != nil || s == "" {
		t.Fatalf("sharpe: %q %v", s, err)
	}
	_, err = orderByForLeaderboardMetric("nosuch")
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestPeriodWhereClause(t *testing.T) {
	_, err := periodWhereClause("all")
	if err != nil {
		t.Fatal(err)
	}
	_, err = periodWhereClause("month")
	if err != nil {
		t.Fatal(err)
	}
	_, err = periodWhereClause("nope")
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestParseLeaderboardLimit(t *testing.T) {
	if parseLeaderboardLimit("") != 20 {
		t.Fatal("default")
	}
	if parseLeaderboardLimit("50") != 50 {
		t.Fatal("50")
	}
	if parseLeaderboardLimit("0") != 20 {
		t.Fatal("invalid to default")
	}
}
