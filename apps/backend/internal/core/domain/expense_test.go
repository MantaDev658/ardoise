package domain

import (
	"errors"
	"testing"

	"ardoise/libs/shared/money"
)

func TestNewExpense_Validation(t *testing.T) {
	total, _ := money.New(3000)
	split10, _ := money.New(1000)
	split20, _ := money.New(2000)

	tests := []struct {
		name        string
		payer       UserID
		splits      []Split
		expectError error
	}{
		{
			name:  "Valid expense",
			payer: "Alice",
			splits: []Split{
				{User: "Alice", Amount: split10},
				{User: "Bob", Amount: split20},
			},
			expectError: nil,
		},
		{
			name:  "Missing payer",
			payer: "",
			splits: []Split{
				{User: "Bob", Amount: total},
			},
			expectError: ErrMissingPayer,
		},
		{
			name:  "Splits don't add up",
			payer: "Alice",
			splits: []Split{
				{User: "Alice", Amount: split10},
				{User: "Bob", Amount: split10},
			},
			expectError: ErrSplitsDoNotEqualTotal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewExpense("exp-1", nil, "Test", total, tt.payer, tt.splits)
			if !errors.Is(err, tt.expectError) {
				t.Errorf("got error %v, want %v", err, tt.expectError)
			}
		})
	}
}

func TestCalculatePairwiseBalance(t *testing.T) {
	t.Run("simple: Bob owes Alice", func(t *testing.T) {
		total, _ := money.New(3000)
		split15, _ := money.New(1500)
		exp, _ := NewExpense("exp-1", nil, "Dinner", total, "Alice", []Split{
			{User: "Alice", Amount: split15},
			{User: "Bob", Amount: split15},
		})
		pairwise := CalculatePairwiseBalance([]*Expense{exp}, "Alice")
		if pairwise["Bob"] != 1500 {
			t.Errorf("Bob→Alice: got %d, want 1500", pairwise["Bob"])
		}
	})

	t.Run("zero-sum trap: Alice has net 0 but live pairwise debts", func(t *testing.T) {
		// Alice pays $50 split with Bob → Bob owes Alice $50
		total50, _ := money.New(5000)
		split25, _ := money.New(2500)
		exp1, _ := NewExpense("exp-1", nil, "Lunch", total50, "Alice", []Split{
			{User: "Alice", Amount: split25},
			{User: "Bob", Amount: split25},
		})

		// Charlie pays $50 split with Alice → Alice owes Charlie $50
		exp2, _ := NewExpense("exp-2", nil, "Dinner", total50, "Charlie", []Split{
			{User: "Charlie", Amount: split25},
			{User: "Alice", Amount: split25},
		})

		pairwise := CalculatePairwiseBalance([]*Expense{exp1, exp2}, "Alice")

		// Aggregate net is 0, but pairwise must show live balances
		if pairwise["Bob"] != 2500 {
			t.Errorf("Bob→Alice: got %d, want 2500", pairwise["Bob"])
		}
		if pairwise["Charlie"] != -2500 {
			t.Errorf("Alice→Charlie: got %d, want -2500", pairwise["Charlie"])
		}
		// Confirm at least one balance is non-zero — Alice must NOT be allowed to leave
		hasOutstanding := false
		for _, v := range pairwise {
			if v != 0 {
				hasOutstanding = true
			}
		}
		if !hasOutstanding {
			t.Error("expected outstanding pairwise balance; zero-sum trap not caught")
		}
	})

	t.Run("fully settled: all pairwise balances are zero", func(t *testing.T) {
		total, _ := money.New(3000)
		split15, _ := money.New(1500)
		// Alice paid, then Bob paid an equal expense back
		exp1, _ := NewExpense("exp-1", nil, "A", total, "Alice", []Split{
			{User: "Alice", Amount: split15},
			{User: "Bob", Amount: split15},
		})
		exp2, _ := NewExpense("exp-2", nil, "B", total, "Bob", []Split{
			{User: "Bob", Amount: split15},
			{User: "Alice", Amount: split15},
		})
		pairwise := CalculatePairwiseBalance([]*Expense{exp1, exp2}, "Alice")
		for other, v := range pairwise {
			if v != 0 {
				t.Errorf("expected settled balance with %s, got %d", other, v)
			}
		}
	})
}

func TestCalculateNetBalances(t *testing.T) {
	// Alice pays $30. Alice and Bob split it ($15 each).
	// Alice should be +15 (owed), Bob should be -15 (owes)
	total, _ := money.New(3000)
	split15, _ := money.New(1500)

	exp, _ := NewExpense("exp-1", nil, "Dinner", total, "Alice", []Split{
		{User: "Alice", Amount: split15},
		{User: "Bob", Amount: split15},
	})

	balances := CalculateNetBalances([]*Expense{exp})

	if balances["Alice"] != 1500 {
		t.Errorf("Alice balance: got %d, want 1500", balances["Alice"])
	}
	if balances["Bob"] != -1500 {
		t.Errorf("Bob balance: got %d, want -1500", balances["Bob"])
	}
}
