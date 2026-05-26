package money_test

import (
	"reflect"
	"testing"

	"ardoise/libs/shared/money"
)

func TestMoney_Distribute(t *testing.T) {
	tests := []struct {
		name   string
		amount money.Money
		n      int
		want   []money.Money
	}{
		{
			name:   "Even split",
			amount: 100,
			n:      2,
			want:   []money.Money{50, 50},
		},
		{
			name:   "Uneven split (remainder 1)",
			amount: 10,
			n:      3,
			want:   []money.Money{4, 3, 3}, // 4+3+3 = 10
		},
		{
			name:   "Uneven split (remainder 2)",
			amount: 11,
			n:      3,
			want:   []money.Money{4, 4, 3}, // 4+4+3 = 11
		},
		{
			name:   "Split 1 way",
			amount: 500,
			n:      1,
			want:   []money.Money{500},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.amount.Distribute(tt.n); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Distribute() = %v, want %v", got, tt.want)
			}
		})
	}
}
