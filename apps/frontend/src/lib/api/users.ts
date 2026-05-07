import { apiFetch } from './client';
import { authStore } from '../stores/auth';
import type { LoginResponse, User } from './types';

export function listUsers() {
	return apiFetch<User[]>('/users');
}

export function listFriends() {
	return apiFetch<User[]>('/friends');
}

export function updateUser(id: string, displayName: string) {
	return apiFetch<void>(`/users/${id}`, {
		method: 'PUT',
		body: JSON.stringify({ display_name: displayName })
	});
}

export function deleteUser(id: string) {
	return apiFetch<void>(`/users/${id}`, { method: 'DELETE' });
}

export function changePassword(id: string, currentPassword: string, newPassword: string) {
	return apiFetch<void>(`/users/${id}/password`, {
		method: 'PUT',
		body: JSON.stringify({ current_password: currentPassword, new_password: newPassword })
	});
}

export async function register(id: string, displayName: string, password: string) {
	await apiFetch<void>('/auth/register', {
		method: 'POST',
		body: JSON.stringify({ id, display_name: displayName, password })
	});
}

export async function login(id: string, password: string) {
	const res = await apiFetch<LoginResponse>('/auth/login', {
		method: 'POST',
		body: JSON.stringify({ id, password })
	});
	authStore.login(res.token);
}

export function logout() {
	authStore.logout();
}
