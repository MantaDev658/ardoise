import { test, expect } from '@playwright/test';
import {
	uniqueUser,
	register,
	loginAs,
	registerViaApi,
	loginViaApi,
	createGroupViaApi,
	inviteUserViaApi,
} from './helpers';

test('empty invitations page shows no-pending message', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await page.goto('/invitations');
	await expect(page.getByText('No pending invitations.')).toBeVisible({ timeout: 15_000 });
});

test('pending invitation appears and can be accepted', async ({ page }) => {
	const owner = uniqueUser();
	const invitee = uniqueUser();

	await registerViaApi(invitee);
	await register(page, owner);

	const token = await loginViaApi(owner);
	const groupID = await createGroupViaApi(token, 'Accept Test Group');
	await inviteUserViaApi(token, groupID, invitee);

	// Log in as the invitee
	await loginAs(page, invitee);
	await page.goto('/invitations');

	// Invitation should be visible
	await expect(page.getByText('Accept Test Group')).toBeVisible({ timeout: 15_000 });
	await expect(page.getByText(new RegExp(`Invited by ${owner.id}`))).toBeVisible();

	// Accept
	await page.getByRole('button', { name: 'ACCEPT' }).click();
	await expect(page.getByRole('alert').getByText(/Joined "Accept Test Group"/)).toBeVisible();

	// Invitation removed from list
	await expect(page.getByText('Accept Test Group')).not.toBeVisible();
	await expect(page.getByText('No pending invitations.')).toBeVisible();

	// Group now appears in the user's groups list
	await page.goto('/groups');
	await expect(page.getByText('Accept Test Group')).toBeVisible({ timeout: 10_000 });
});

test('pending invitation can be declined', async ({ page }) => {
	const owner = uniqueUser();
	const invitee = uniqueUser();

	await registerViaApi(invitee);
	await register(page, owner);

	const token = await loginViaApi(owner);
	const groupID = await createGroupViaApi(token, 'Decline Test Group');
	await inviteUserViaApi(token, groupID, invitee);

	await loginAs(page, invitee);
	await page.goto('/invitations');

	await expect(page.getByText('Decline Test Group')).toBeVisible({ timeout: 15_000 });

	await page.getByRole('button', { name: 'DECLINE' }).click();
	await expect(page.getByRole('alert').getByText('Invitation declined.')).toBeVisible();

	await expect(page.getByText('Decline Test Group')).not.toBeVisible();
	await expect(page.getByText('No pending invitations.')).toBeVisible();

	// Group does NOT appear in the user's groups list
	await page.goto('/groups');
	await expect(page.getByText('Decline Test Group')).not.toBeVisible();
});

test('invitations nav link is visible and navigates to the page', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await expect(page.getByRole('link', { name: 'INVITATIONS' })).toBeVisible();
	await page.getByRole('link', { name: 'INVITATIONS' }).click();
	await expect(page).toHaveURL('/invitations');
	await expect(page.getByText('No pending invitations.')).toBeVisible({ timeout: 15_000 });
});
