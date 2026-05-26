package csv

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"ardoise/apps/backend/internal/core/domain"
	"ardoise/libs/shared/money"

	"github.com/google/uuid"
)

// ParseExpenses reads a CSV ledger file and returns the validated expenses it contains.
func ParseExpenses(filePath string) ([]*domain.Expense, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // allow dynamic columns

	var expenses []*domain.Expense
	lineNum := 0

	for {
		lineNum++
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading row %d: %w", lineNum, err)
		}

		if len(record) < 7 {
			return nil, fmt.Errorf("row %d has insufficient columns", lineNum)
		}

		desc := record[1]

		cents, err := strconv.ParseInt(record[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid total amount on row %d", lineNum)
		}
		totalMoney, _ := money.New(cents)

		payer := domain.UserID(strings.TrimSpace(record[4]))
		strategy := strings.ToUpper(strings.TrimSpace(record[5]))
		participantData := record[6:]

		var splits []domain.Split

		switch strategy {
		case "EVEN":
			splits = parseEven(participantData, totalMoney)
		case "EXACT":
			splits, err = parseExact(participantData)
		case "PERCENT", "SHARES":
			splits, err = parseWeighted(participantData, totalMoney)
		default:
			return nil, fmt.Errorf("unknown strategy '%s' on row %d", strategy, lineNum)
		}

		if err != nil {
			return nil, fmt.Errorf("error parsing splits on row %d: %w", lineNum, err)
		}

		expense, err := domain.NewExpense(
			domain.ExpenseID(uuid.NewString()),
			nil,
			desc,
			totalMoney,
			payer,
			splits,
		)
		if err != nil {
			return nil, fmt.Errorf("domain validation failed on row %d: %w", lineNum, err)
		}

		expenses = append(expenses, expense)
	}

	return expenses, nil
}

func parseEven(participants []string, total money.Money) []domain.Split {
	splitAmounts := total.Distribute(len(participants))
	var splits []domain.Split
	for i, name := range participants {
		splits = append(splits, domain.Split{
			User:   domain.UserID(strings.TrimSpace(name)),
			Amount: splitAmounts[i],
		})
	}
	return splits
}

func parseExact(participantData []string) ([]domain.Split, error) {
	var splits []domain.Split
	for _, p := range participantData {
		parts := strings.Split(p, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid exact format (expected User:Cents), got %s", p)
		}

		user := domain.UserID(strings.TrimSpace(parts[0]))
		cents, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			return nil, err
		}
		amt, _ := money.New(cents)
		splits = append(splits, domain.Split{User: user, Amount: amt})
	}
	return splits, nil
}

func parseWeighted(participantData []string, total money.Money) ([]domain.Split, error) {
	type weightedUser struct {
		user   domain.UserID
		weight int64
	}
	var users []weightedUser
	var totalWeight int64 = 0

	for _, p := range participantData {
		parts := strings.Split(p, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid weighted format (expected User:Weight), got %s", p)
		}

		weight, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			return nil, err
		}
		users = append(users, weightedUser{
			user:   domain.UserID(strings.TrimSpace(parts[0])),
			weight: weight,
		})
		totalWeight += weight
	}

	var splits []domain.Split
	var allocatedCents int64 = 0
	totalCents := total.Int64()

	for _, u := range users {
		shareCents := (totalCents * u.weight) / totalWeight
		amt, _ := money.New(shareCents)
		splits = append(splits, domain.Split{User: u.user, Amount: amt})
		allocatedCents += shareCents
	}

	remainder := totalCents - allocatedCents
	for i := 0; i < int(remainder); i++ {
		currentAmt := splits[i].Amount.Int64()
		splits[i].Amount, _ = money.New(currentAmt + 1)
	}

	return splits, nil
}
