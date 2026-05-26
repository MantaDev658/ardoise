package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"ardoise/apps/backend/internal/core/domain"
	"ardoise/apps/backend/internal/core/infrastructure/csv"
)

func main() {
	filePath := flag.String("file", "", "Path to the CSV file containing expenses")
	noSimplify := flag.Bool("no-simplify", false, "Show raw net balances instead of simplified transactions")
	flag.Parse()

	if *filePath == "" {
		fmt.Fprintln(os.Stderr, "Usage: ardoise-cli -file=<path_to_csv> [--no-simplify]")
		os.Exit(1)
	}

	expenses, err := csv.ParseExpenses(*filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error parsing CSV: %v\n", err)
		os.Exit(1)
	}

	netBalances := domain.CalculateNetBalances(expenses)

	if *noSimplify {
		printRawLedger(netBalances)
	} else {
		printSimplifiedTransactions(netBalances)
	}
}

func printSimplifiedTransactions(balances map[domain.UserID]int64) {
	transactions := domain.SimplifyDebts(balances)

	fmt.Println("\n--- 💸 Ardoise: Settle Up Instructions ---")
	if len(transactions) == 0 {
		fmt.Println("🎉 Everyone is completely settled up!")
	} else {
		for _, tx := range transactions {
			fmt.Printf("➡️  %-10s pays %-10s $%7.2f\n", tx.From, tx.To, float64(tx.Amount)/100.0)
		}
	}
	fmt.Println("-------------------------------------------")
}

func printRawLedger(balances map[domain.UserID]int64) {
	fmt.Println("\n--- 📊 Ardoise: Raw Net Balances ---")
	var users []domain.UserID
	for user := range balances {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool { return users[i] < users[j] })

	for _, user := range users {
		balance := float64(balances[user]) / 100.0
		switch {
		case balance > 0:
			fmt.Printf("✅ %-10s is OWED:   $%7.2f\n", user, balance)
		case balance < 0:
			fmt.Printf("🔴 %-10s OWES:       $%7.2f\n", user, -balance)
		default:
			fmt.Printf("⚪ %-10s is SETTLED: $%7.2f\n", user, balance)
		}
	}
	fmt.Println("---------------------------------------")
}
