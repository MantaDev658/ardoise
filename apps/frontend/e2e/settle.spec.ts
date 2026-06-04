import { test, expect } from '@playwright/test';
import {
	uniqueUser,
	register,
	registerViaApi,
	loginViaApi,
	createGroupViaApi,
	addGroupMemberViaApi,
} from './helpers';

test('settle page shows empty state when user has no group friends', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await page.goto('/settle');

	await expect(
		page.getByText('You have no friends yet — join a group first.')
	).toBeVisible({ timeout: 15_000 });
});

test('settle up records a payment and shows success toast', async ({ page }) => {
	const recipient = uniqueUser();
	const payer = uniqueUser();

	// Register both users and put them in a group together
	await registerViaApi(recipient);
	await register(page, payer);
	const token = await loginViaApi(payer);
	const groupID = await createGroupViaApi(token, 'Settle Test Group');
	await addGroupMemberViaApi(token, groupID, recipient);

	await page.goto('/settle');

	// Wait for the select to become enabled (loading done)
	await expect(page.locator('#settle-receiver')).toBeEnabled({ timeout: 15_000 });

	// Select the recipient by their username (user ID)
	await page.locator('#settle-receiver').selectOption({ value: recipient.id });

	// Enter amount and submit
	await page.fill('#settle-amount', '25.00');
	await page.getByRole('button', { name: 'SETTLE UP' }).click();

	await expect(page.getByRole('alert').getByText('Settlement recorded!')).toBeVisible();
});

test('settle form validates missing recipient', async ({ page }) => {
	const recipient = uniqueUser();
	const payer = uniqueUser();

	// Put them in a group so the form is visible
	await registerViaApi(recipient);
	await register(page, payer);
	const token = await loginViaApi(payer);
	const groupID = await createGroupViaApi(token, 'Settle Validation Group');
	await addGroupMemberViaApi(token, groupID, recipient);

	await page.goto('/settle');

	// Wait for the form to be visible and enabled
	await expect(page.locator('#settle-amount')).toBeEnabled({ timeout: 15_000 });

	// Submit without filling in a recipient
	await page.fill('#settle-amount', '10.00');
	await page.getByRole('button', { name: 'SETTLE UP' }).click();

	await expect(page.getByRole('alert').getByText('Select a recipient.')).toBeVisible();
});
