import { apiFetch } from './client';
import type {
	BalancesResponse,
	ExpenseItem,
	FriendBalance,
	Paginated,
	SplitInput,
	SplitType
} from './types';

export interface CreateExpenseInput {
	group_id?: string;
	description: string;
	total_cents: number;
	split_type: SplitType;
	splits: SplitInput[];
}

export interface UpdateExpenseInput {
	description: string;
	total_cents: number;
	payer: string;
	split_type: SplitType;
	splits: SplitInput[];
	group_id?: string;
}

export function listExpenses(groupID?: string, cursor?: string, limit = 20) {
	const params = new URLSearchParams();
	if (groupID) params.set('group_id', groupID);
	if (cursor) params.set('cursor', cursor);
	params.set('limit', String(limit));
	return apiFetch<Paginated<ExpenseItem>>(`/expenses?${params}`);
}

export function createExpense(input: CreateExpenseInput) {
	return apiFetch<void>('/expenses', { method: 'POST', body: JSON.stringify(input) });
}

export function updateExpense(id: string, input: UpdateExpenseInput) {
	return apiFetch<{ status: string }>(`/expenses/${id}`, {
		method: 'PUT',
		body: JSON.stringify(input)
	});
}

export function deleteExpense(id: string) {
	return apiFetch<{ status: string }>(`/expenses/${id}`, { method: 'DELETE' });
}

export function getBalances(groupID?: string) {
	const params = groupID ? `?group_id=${groupID}` : '';
	return apiFetch<BalancesResponse>(`/balances${params}`);
}

export function getFriendBalances(userID: string) {
	return apiFetch<FriendBalance[]>(`/friends/${userID}/balances`);
}
