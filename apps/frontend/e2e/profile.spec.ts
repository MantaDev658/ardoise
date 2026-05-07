import { test, expect } from '@playwright/test';
import { uniqueUser, register, loginAs } from './helpers';

test('profile page is reachable via nav username link', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await page.getByRole('link', { name: user.id }).click();
	await page.waitForURL('/profile');
	await expect(page.getByText('PROFILE', { exact: true })).toBeVisible();
});

test('change password with wrong current password shows error', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await page.goto('/profile');

	await page.fill('#profile-current-pw', 'wrong-password');
	await page.fill('#profile-new-pw', 'newpassword123');
	await page.fill('#profile-confirm-pw', 'newpassword123');
	await page.getByRole('button', { name: 'CHANGE PASSWORD' }).click();

	await expect(page.getByText(/current password is incorrect/i)).toBeVisible({ timeout: 5_000 });
});

test('change password with mismatched confirm shows client error', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await page.goto('/profile');

	await page.fill('#profile-current-pw', user.password);
	await page.fill('#profile-new-pw', 'newpassword123');
	await page.fill('#profile-confirm-pw', 'doesnotmatch');
	await page.getByRole('button', { name: 'CHANGE PASSWORD' }).click();

	await expect(page.getByText(/do not match/i)).toBeVisible({ timeout: 5_000 });
});

test('change password with too-short new password shows error', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await page.goto('/profile');

	await page.fill('#profile-current-pw', user.password);
	await page.fill('#profile-new-pw', 'short');
	await page.fill('#profile-confirm-pw', 'short');
	await page.getByRole('button', { name: 'CHANGE PASSWORD' }).click();

	await expect(page.getByText(/at least 8/i)).toBeVisible({ timeout: 5_000 });
});

test('successful password change clears fields and allows login with new password', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await page.goto('/profile');

	const newPassword = 'newSecurePass99';
	await page.fill('#profile-current-pw', user.password);
	await page.fill('#profile-new-pw', newPassword);
	await page.fill('#profile-confirm-pw', newPassword);
	await page.getByRole('button', { name: 'CHANGE PASSWORD' }).click();

	await expect(page.getByText(/password changed/i)).toBeVisible({ timeout: 5_000 });

	// Fields should be cleared
	await expect(page.locator('#profile-current-pw')).toHaveValue('');
	await expect(page.locator('#profile-new-pw')).toHaveValue('');

	// Can log in with the new password
	await page.evaluate(() => localStorage.clear());
	await loginAs(page, { ...user, password: newPassword });
	await expect(page).toHaveURL('/');
});
