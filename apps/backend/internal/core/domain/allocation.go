package domain

import (
	"math"

	"ardoise/libs/shared/money"
)

// AllocationType identifies which split algorithm to apply.
type AllocationType string

const (
	AllocationTypeExact      AllocationType = "EXACT"
	AllocationTypeEqual      AllocationType = "EQUAL"
	AllocationTypePercentage AllocationType = "PERCENTAGE"
	AllocationTypeShares     AllocationType = "SHARES"
)

// AllocationInput is the per-participant data passed to Allocate.
type AllocationInput struct {
	UserID UserID
	Value  float64
}

// Allocate distributes totalCents among participants using the given strategy.
func Allocate(strategy AllocationType, totalCents int64, participants []AllocationInput) ([]Split, error) {
	if len(participants) == 0 {
		return nil, ErrParticipantNotFound
	}

	var result []Split

	switch strategy {
	case AllocationTypeExact:
		var calculatedSum int64 = 0
		for _, p := range participants {
			cents := int64(p.Value)
			m, _ := money.New(cents)
			result = append(result, Split{User: p.UserID, Amount: m})
			calculatedSum += cents
		}
		if calculatedSum != totalCents {
			return nil, ErrSplitsDoNotEqualTotal
		}
		return result, nil

	case AllocationTypeEqual, AllocationTypeShares, AllocationTypePercentage:
		type shareCalc struct {
			input  AllocationInput
			shares int64
			cents  int64
		}
		var calcs []*shareCalc
		var totalShares int64 = 0

		for _, p := range participants {
			var s int64
			switch strategy {
			case AllocationTypeEqual:
				s = 1
			case AllocationTypeShares:
				s = int64(p.Value)
			case AllocationTypePercentage:
				s = int64(math.Round(p.Value * 100))
			}

			totalShares += s
			calcs = append(calcs, &shareCalc{input: p, shares: s})
		}

		if strategy == AllocationTypePercentage && totalShares != 10000 {
			return nil, ErrInvalidPercentages
		}
		if totalShares <= 0 {
			return nil, ErrInvalidNumberOfShares
		}

		var distributed int64 = 0
		for _, c := range calcs {
			c.cents = (totalCents * c.shares) / totalShares
			distributed += c.cents
		}

		remainder := totalCents - distributed
		for i := 0; int64(i) < remainder; i++ {
			calcs[i].cents++
		}

		for _, c := range calcs {
			m, _ := money.New(c.cents)
			result = append(result, Split{User: c.input.UserID, Amount: m})
		}
		return result, nil

	default:
		return nil, ErrInvalidAllocationStrategy
	}
}
