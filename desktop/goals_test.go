package desktop

import (
	"strings"
	"testing"
)

func TestGoalLifecycle(t *testing.T) {
	api := newTestAPI(t)

	if _, err := api.CreateGoal(NewGoalInput{Name: "Car", Currency: "GBP", Target: "100", Accumulated: "0", StartedAt: "2026-01-01"}); err == nil || !strings.Contains(err.Error(), "unknown currency") {
		t.Fatalf("expected unknown currency, got %v", err)
	}

	created, err := api.CreateGoal(NewGoalInput{Name: "Car", Currency: "eur", Target: "1000", Accumulated: "100", StartedAt: "2026-01-01", TargetDate: "2020-01-01"})
	if err != nil || created.Mode != "active" {
		t.Fatalf("create: %+v %v", created, err)
	}

	created, err = api.CreateGoal(NewGoalInput{Name: "Phone", Currency: "$", Target: "50", Accumulated: "50", StartedAt: "2026-01-01"})
	if err != nil || created.Mode != "done" {
		t.Fatalf("a reached goal should land in done: %+v %v", created, err)
	}

	view, err := api.Goals("active")
	if err != nil {
		t.Fatal(err)
	}

	if len(view.Goals) != 1 || view.Counts["active"] != 1 || view.Counts["done"] != 1 {
		t.Fatalf("unexpected lists %+v %+v", view.Goals, view.Counts)
	}

	car := view.Goals[0]
	if car.Currency != "EUR" || car.LeftCents != 90000 || car.Percent != 10 || !car.Overdue || car.StartedAt != "2026-01-01" {
		t.Fatalf("unexpected row %+v", car)
	}

	if view.Progress.PaidCents != 10800 || view.Progress.TotalCents != 108000 {
		t.Fatalf("expected progress in base, got %+v", view.Progress)
	}

	row, err := api.AddGoalChange(PaymentInput{ID: car.ID, Delta: "+400", Date: "2026-02-01", Note: "bonus"})
	if err != nil || row.AccumulatedCents != 50000 {
		t.Fatalf("change: %+v %v", row, err)
	}

	logs, err := api.GoalLogs(car.ID)
	if err != nil || len(logs) != 1 || logs[0].DeltaCents != 40000 || logs[0].Note != "bonus" {
		t.Fatalf("logs: %+v %v", logs, err)
	}

	if err := api.UpdateGoal(GoalUpdateInput{ID: car.ID, Target: "1000", Accumulated: "1000", StartedAt: "2026-01-01", Description: " done "}); err != nil {
		t.Fatal(err)
	}

	view, _ = api.Goals("done")
	if view.Counts["done"] != 2 || view.Counts["active"] != 0 {
		t.Fatalf("expected car done, got %+v", view.Counts)
	}

	row, err = api.DeleteGoalChange(car.ID, logs[0].ID)
	if err != nil || row.AccumulatedCents != 60000 {
		t.Fatalf("delete change should undo it: %+v %v", row, err)
	}

	if logs, _ := api.GoalLogs(car.ID); len(logs) != 0 {
		t.Fatalf("log should be gone: %+v", logs)
	}

	if _, err := api.DeleteGoalChange(car.ID, logs[0].ID); err != errLogNotFound {
		t.Fatalf("expected log not found, got %v", err)
	}

	if err := api.DeleteGoal(car.ID); err != nil {
		t.Fatal(err)
	}

	if err := api.DeleteGoal(car.ID); err != errGoalNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}
