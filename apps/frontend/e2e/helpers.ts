import type { Page } from '@playwright/test';

export interface TestUser {
	id: string;
	displayName: string;
	password: string;
}

export function uniqueUser(): TestUser {
	const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
	return {
		id: `tester-${suffix}`,
		displayName: `Tester ${suffix}`,
		password: 'testpass123',
	};
}

export async function register(page: Page, user: TestUser): Promise<void> {
	await page.goto('/register');
	await page.fill('#reg-id', user.id);
	await page.fill('#reg-display-name', user.displayName);
	await page.fill('#reg-password', user.password);
	await page.getByRole('button', { name: 'REGISTER' }).click();
	await page.waitForURL('/');
	// Wait for the dashboard's initial data load to finish so we don't navigate
	// away while loadDashboard is still in flight (which would leave async state
	// mutations targeting a destroyed component, causing null-dereference errors
	// in the next page's reactive graph).
	await page.waitForLoadState('networkidle');
}

export async function loginAs(page: Page, user: TestUser): Promise<void> {
	await page.goto('/login');
	await page.fill('#login-id', user.id);
	await page.fill('#login-password', user.password);
	await page.getByRole('button', { name: 'LOGIN' }).click();
	await page.waitForURL('/');
	await page.waitForLoadState('networkidle');
}

// Register a user directly via the backend API (no browser interaction needed).
// Useful for creating a second user that just needs to exist in the DB.
export async function registerViaApi(user: TestUser): Promise<void> {
	const res = await fetch('http://localhost:8080/auth/register', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ id: user.id, display_name: user.displayName, password: user.password }),
	});
	if (!res.ok) throw new Error(`registerViaApi failed: ${res.status} ${await res.text()}`);
}

export async function loginViaApi(user: TestUser): Promise<string> {
	const res = await fetch('http://localhost:8080/auth/login', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ id: user.id, password: user.password }),
	});
	if (!res.ok) throw new Error(`loginViaApi failed: ${res.status} ${await res.text()}`);
	const data = await res.json() as { token: string };
	return data.token;
}

export async function createGroupExpenseViaApi(
	token: string,
	groupID: string,
	participantIDs: string[],
	totalCents: number,
): Promise<void> {
	const splits = participantIDs.map((id) => ({ user_id: id, value: totalCents / participantIDs.length }));
	const res = await fetch('http://localhost:8080/expenses', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify({
			group_id: groupID,
			description: 'Test expense',
			total_cents: totalCents,
			split_type: 'EQUAL',
			splits,
		}),
	});
	if (!res.ok) throw new Error(`createGroupExpenseViaApi failed: ${res.status} ${await res.text()}`);
}

export async function createGroupViaApi(
	token: string,
	name: string,
): Promise<string> {
	const res = await fetch('http://localhost:8080/groups', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify({ name }),
	});
	if (!res.ok) throw new Error(`createGroupViaApi failed: ${res.status} ${await res.text()}`);
	const data = await res.json() as { group_id: string };
	return data.group_id;
}

export async function addGroupMemberViaApi(
	token: string,
	groupID: string,
	userID: string,
): Promise<void> {
	const res = await fetch(`http://localhost:8080/groups/${groupID}/members`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: JSON.stringify({ user_id: userID }),
	});
	if (!res.ok) throw new Error(`addGroupMemberViaApi failed: ${res.status} ${await res.text()}`);
}
