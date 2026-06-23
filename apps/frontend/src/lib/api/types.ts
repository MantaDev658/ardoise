// Matches backend domain.User (no json tags — Go uses field names directly)
export interface User {
	ID: string;
	DisplayName: string;
	IsActive: boolean;
}

// Matches backend domain.Group (no json tags)
export interface Group {
	ID: string;
	Name: string;
	Members: string[]; // array of UserIDs
}

// Matches backend domain.FriendBalance (no json tags)
export interface FriendBalance {
	FriendID: string;
	NetCents: number; // positive = they owe you, negative = you owe them
}

// Matches backend domain.Transaction (no json tags)
export interface SettlementSuggestion {
	From: string;
	To: string;
	Amount: number; // cents
}

export interface ExpenseSplit {
	user_id: string;
	amount_cents: number;
}

// Matches inline expenseItem struct in ListExpenses handler
export interface ExpenseItem {
	id: string;
	group_id?: string;
	description: string;
	total_cents: number;
	payer: string;
	created_at: string;
	splits: ExpenseSplit[];
}

// GET /balances response
export interface BalancesResponse {
	net_balances: Record<string, number>;
	suggested_settlements: SettlementSuggestion[];
}

// Paginated list response (GET /expenses)
export interface Paginated<T> {
	data: T[];
	next_cursor: string;
}

// POST /auth/login response
export interface LoginResponse {
	token: string;
}

// POST /groups response
export interface CreateGroupResponse {
	status: string;
	group_id: string;
}

// Matches backend domain.Invitation (no json tags)
export interface Invitation {
	ID: string;
	GroupID: string;
	GroupName: string;
	InviterID: string;
	InviteeID: string;
	CreatedAt: string;
}

export type SplitType = 'EQUAL' | 'EXACT' | 'PERCENTAGE' | 'SHARES';

export interface SplitInput {
	user_id: string;
	value?: number;
}
