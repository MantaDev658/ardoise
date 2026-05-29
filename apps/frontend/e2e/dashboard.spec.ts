import { test, expect } from '@playwright/test';
import { uniqueUser, register } from './helpers';

test('new user sees welcome screen on dashboard', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	// register() already navigates to '/' and waits for networkidle
	await expect(page.getByText('WELCOME TO ARDOISE', { exact: true })).toBeVisible({ timeout: 10_000 });
});
