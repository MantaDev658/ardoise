package domain

import (
	"errors"
	"testing"
)

func TestAllocate_Equal(t *testing.T) {
	// $10.00 split 3 ways. Should be $3.34, $3.33, $3.33
	inputs := []AllocationInput{
		{UserID: "Alice"}, {UserID: "Bob"}, {UserID: "Charlie"},
	}

	splits, err := Allocate(AllocationTypeEqual, 1000, inputs)
	if err != nil {
		t.Fatal(err)
	}

	if splits[0].Amount.Int64() != 334 || splits[1].Amount.Int64() != 333 || splits[2].Amount.Int64() != 333 {
		t.Errorf("Equal penny rounding failed, got: %v", splits)
	}
}

func TestAllocate_Percentage(t *testing.T) {
	t.Run("clean integer percentages", func(t *testing.T) {
		inputs := []AllocationInput{
			{UserID: "Alice", Value: 60.00},
			{UserID: "Bob", Value: 40.00},
		}
		splits, err := Allocate(AllocationTypePercentage, 1000, inputs)
		if err != nil {
			t.Fatal(err)
		}
		if splits[0].Amount.Int64() != 600 || splits[1].Amount.Int64() != 400 {
			t.Errorf("Percentage split failed, got %v", splits)
		}
	})

	t.Run("fractional percentages summing to 100 are accepted", func(t *testing.T) {
		// 33.33 + 33.33 + 33.34 = 100.00 — previously rejected due to float64 truncation
		inputs := []AllocationInput{
			{UserID: "Alice", Value: 33.33},
			{UserID: "Bob", Value: 33.33},
			{UserID: "Charlie", Value: 33.34},
		}
		splits, err := Allocate(AllocationTypePercentage, 10000, inputs)
		if err != nil {
			t.Fatalf("expected no error for valid fractional percentages, got: %v", err)
		}
		var total int64
		for _, s := range splits {
			total += s.Amount.Int64()
		}
		if total != 10000 {
			t.Errorf("splits do not sum to total: got %d, want 10000", total)
		}
	})

	t.Run("percentages not summing to 100 are rejected", func(t *testing.T) {
		inputs := []AllocationInput{
			{UserID: "Alice", Value: 50.00},
			{UserID: "Bob", Value: 40.00},
		}
		_, err := Allocate(AllocationTypePercentage, 1000, inputs)
		if !errors.Is(err, ErrInvalidPercentages) {
			t.Errorf("expected ErrInvalidPercentages, got %v", err)
		}
	})
}
