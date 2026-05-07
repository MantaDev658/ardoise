import { test, expect } from '@playwright/test';
import { uniqueUser, register, loginAs } from './helpers';

test('register creates account and redirects to dashboard', async ({ page }) => {
	const user = uniqueUser();
	await page.goto('/register');
	await page.fill('#reg-id', user.id);
	await page.fill('#reg-display-name', user.displayName);
	await page.fill('#reg-password', user.password);
	await page.getByRole('button', { name: 'REGISTER' }).click();
	await page.waitForURL('/');
	// Nav brand text ("OPEN SPLIT") confirms the authenticated layout rendered
	await expect(page.getByText('OPEN SPLIT', { exact: true })).toBeVisible();
});

test('login with valid credentials redirects to dashboard', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await page.evaluate(() => localStorage.clear());
	await loginAs(page, user);
	await expect(page).toHaveURL('/');
	await expect(page.getByText('OPEN SPLIT', { exact: true })).toBeVisible();
});

test('registering a duplicate username shows an error', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await page.evaluate(() => localStorage.clear());

	// Try to register the same username again
	await page.goto('/register');
	await page.fill('#reg-id', user.id);
	await page.fill('#reg-display-name', 'Someone Else');
	await page.fill('#reg-password', 'otherpass123');
	await page.getByRole('button', { name: 'REGISTER' }).click();

	await expect(page.getByText(/username already taken/i)).toBeVisible();
	await expect(page).toHaveURL('/register');
});

test('login with bad credentials shows error', async ({ page }) => {
	await page.goto('/login');
	await page.fill('#login-id', 'no-such-user');
	await page.fill('#login-password', 'wrongpassword');
	await page.getByRole('button', { name: 'LOGIN' }).click();
	// The error is rendered as "⚠ Invalid username or password." — match by substring
	await expect(page.getByText(/Invalid username or password/)).toBeVisible();
});

test('unauthenticated user is redirected to /login', async ({ page }) => {
	await page.goto('/expenses');
	await page.waitForURL('**/login');
	await expect(page).toHaveURL(/\/login/);
});

test('already logged-in user is redirected away from /login', async ({ page }) => {
	const user = uniqueUser();
	await register(page, user);
	await page.goto('/login');
	await page.waitForURL('/');
	await expect(page).toHaveURL('/');
});
