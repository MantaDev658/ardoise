import { test, expect } from '@playwright/test';
import {
	uniqueUser,
	register,
	registerViaApi,
	loginViaApi,
	createGroupViaApi,
	addGroupMemberViaApi,
	createGroupExpenseViaApi,
} from './helpers';

test('empty groups page shows no groups message', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await page.goto('/groups');
	await expect(page.getByText('No groups yet. Create one above.')).toBeVisible({ timeout: 15_000 });
});

test('create group appears in list', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await page.goto('/groups');
	await expect(page.getByText('No groups yet. Create one above.')).toBeVisible({ timeout: 15_000 });
	await page.getByRole('button', { name: '+ CREATE GROUP' }).click();
	await page.fill('[placeholder="Group name…"]', 'E2E Test Group');
	await page.getByRole('button', { name: 'CREATE' }).click();
	await expect(page.getByText('E2E Test Group')).toBeVisible();
});

test('rename group updates the name', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await page.goto('/groups');
	await expect(page.getByText('No groups yet. Create one above.')).toBeVisible({ timeout: 15_000 });

	// Create
	await page.getByRole('button', { name: '+ CREATE GROUP' }).click();
	await page.fill('[placeholder="Group name…"]', 'Old Name');
	await page.getByRole('button', { name: 'CREATE' }).click();
	await expect(page.getByText('Old Name')).toBeVisible();

	// Open group detail
	await page.getByText('Old Name').first().click();
	await page.waitForURL(/\/groups\/.+/);

	// Rename
	await page.getByRole('button', { name: 'RENAME' }).click();
	await page.fill('[placeholder="New name…"]', 'Renamed Group');
	await page.getByRole('button', { name: 'SAVE' }).click();

	// After save, the Window title updates to the new name
	await expect(page.getByText('Renamed Group')).toBeVisible();
});

test('delete group redirects to /groups and removes it from list', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await page.goto('/groups');
	await expect(page.getByText('No groups yet. Create one above.')).toBeVisible({ timeout: 15_000 });

	// Create
	await page.getByRole('button', { name: '+ CREATE GROUP' }).click();
	await page.fill('[placeholder="Group name…"]', 'To Be Deleted');
	await page.getByRole('button', { name: 'CREATE' }).click();
	await expect(page.getByText('To Be Deleted')).toBeVisible();

	// Open group detail
	await page.getByText('To Be Deleted').first().click();
	await page.waitForURL(/\/groups\/.+/);

	// Accept the confirm dialog and delete
	page.once('dialog', (dialog) => dialog.accept());
	await page.getByRole('button', { name: 'DELETE' }).click();

	await page.waitForURL('/groups');
	await expect(page).toHaveURL('/groups');
	await expect(page.getByText('To Be Deleted')).not.toBeVisible();
});

test('leave group redirects to /groups', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await page.goto('/groups');
	await expect(page.getByText('No groups yet. Create one above.')).toBeVisible({ timeout: 15_000 });

	// Create a group
	await page.getByRole('button', { name: '+ CREATE GROUP' }).click();
	await page.fill('[placeholder="Group name…"]', 'Group To Leave');
	await page.getByRole('button', { name: 'CREATE' }).click();
	await expect(page.getByText('Group To Leave')).toBeVisible();

	// Open group detail
	await page.getByText('Group To Leave').first().click();
	await page.waitForURL(/\/groups\/.+/);

	// REMOVE button should not be visible for the current user
	// (they are the only member, so no REMOVE buttons at all)
	await expect(page.getByRole('button', { name: 'REMOVE' })).not.toBeVisible();

	// Leave the group
	page.once('dialog', (dialog) => dialog.accept());
	await page.getByRole('button', { name: 'LEAVE' }).click();

	await page.waitForURL('/groups');
	await expect(page).toHaveURL('/groups');
});

test('leave group with outstanding balance shows error', async ({ page }) => {
	const user = uniqueUser();
	const friend = uniqueUser();

	// Set up via API: register both users, create group, add member, create expense
	await registerViaApi(friend);
	await register(page, user); // registers + logs in as user
	const token = await loginViaApi(user);
	const groupID = await createGroupViaApi(token, 'Balance Group');
	await addGroupMemberViaApi(token, groupID, friend.id);
	// User pays $20 split equally with friend — user has +$10 outstanding balance
	await createGroupExpenseViaApi(token, groupID, [user.id, friend.id], 2000);

	// Navigate to the group detail page
	await page.goto(`/groups/${groupID}`);
	await page.waitForURL(/\/groups\/.+/);
	await expect(page.getByRole('button', { name: 'LEAVE' })).toBeVisible({ timeout: 10_000 });

	// Try to leave — backend blocks due to outstanding balance
	page.once('dialog', (dialog) => dialog.accept());
	await page.getByRole('button', { name: 'LEAVE' }).click();

	// Should show error toast and stay on the group page
	await expect(page.getByText(/outstanding/i)).toBeVisible({ timeout: 5_000 });
	await expect(page).toHaveURL(/\/groups\/.+/);
});

test('add member via username search', async ({ page }) => {
	const owner = uniqueUser();
	const newMember = uniqueUser();

	await registerViaApi(newMember);
	await register(page, owner);

	// Create a group via UI
	await page.goto('/groups');
	await expect(page.getByText('No groups yet. Create one above.')).toBeVisible({ timeout: 15_000 });
	await page.getByRole('button', { name: '+ CREATE GROUP' }).click();
	await page.fill('[placeholder="Group name…"]', 'Member Search Group');
	await page.getByRole('button', { name: 'CREATE' }).click();
	await page.getByText('Member Search Group').first().click();
	await page.waitForURL(/\/groups\/.+/);

	// Type the new member's username in the search input and add them
	await page.fill('[placeholder="Search username…"]', newMember.id);
	await page.getByRole('button', { name: '+ ADD' }).click();

	await expect(page.getByRole('alert').getByText('Member added.')).toBeVisible();
	// The new member should now appear in the member list
	await expect(page.getByText(newMember.displayName)).toBeVisible();
});

test('balances tab shows net balances and suggested transfers', async ({ page }) => {
	const user = uniqueUser();
	const friend = uniqueUser();
	await registerViaApi(friend);
	await register(page, user);
	const token = await loginViaApi(user);
	const groupID = await createGroupViaApi(token, 'Balance Tab Group');
	await addGroupMemberViaApi(token, groupID, friend.id);
	// User pays $20, split equally — friend owes user $10
	await createGroupExpenseViaApi(token, groupID, [user.id, friend.id], 2000);

	await page.goto(`/groups/${groupID}`);
	await page.waitForURL(/\/groups\/.+/);

	// Open the Balances tab
	await page.getByRole('button', { name: 'balances' }).click();

	// Net balances section should be visible
	await expect(page.getByText('Net Balances')).toBeVisible({ timeout: 10_000 });

	// Suggested transfers section and SETTLE button should appear
	await expect(page.getByText('Suggested Transfers')).toBeVisible();
	await expect(page.getByRole('link', { name: 'SETTLE', exact: true })).toBeVisible();

	// SETTLE link pre-fills the settle form
	await page.getByRole('link', { name: 'SETTLE', exact: true }).click();
	await page.waitForURL(/\/settle/);
	await expect(page.getByLabel('Amount ($)')).toHaveValue('10.00', { timeout: 15_000 });
});
