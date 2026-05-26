import { writable, get } from 'svelte/store';

interface AuthState {
	token: string | null;
	userID: string | null;
}

// Decodes the `sub` claim from a JWT without verifying the signature.
// Verification is the server's job — we just need the user ID for display logic.
function jwtSub(token: string): string | null {
	try {
		const payload = token.split('.')[1];
		if (!payload) return null;
		const decoded = JSON.parse(atob(payload.replace(/-/g, '+').replace(/_/g, '/')));
		return typeof decoded.sub === 'string' ? decoded.sub : null;
	} catch {
		return null;
	}
}

function loadFromStorage(): AuthState {
	if (typeof localStorage === 'undefined') return { token: null, userID: null };
	try {
		const raw = localStorage.getItem('ardoise_auth');
		return raw ? (JSON.parse(raw) as AuthState) : { token: null, userID: null };
	} catch {
		return { token: null, userID: null };
	}
}

function saveToStorage(state: AuthState): void {
	if (typeof localStorage === 'undefined') return;
	localStorage.setItem('ardoise_auth', JSON.stringify(state));
}

function createAuthStore() {
	const { subscribe, set } = writable<AuthState>(loadFromStorage());

	return {
		subscribe,
		login(token: string): void {
			const state: AuthState = { token, userID: jwtSub(token) };
			saveToStorage(state);
			set(state);
		},
		logout(): void {
			if (typeof localStorage !== 'undefined') {
				localStorage.removeItem('ardoise_auth');
			}
			set({ token: null, userID: null });
		}
	};
}

export const authStore = createAuthStore();

// Convenience getter used outside of reactive Svelte context (e.g. apiFetch)
export function getToken(): string | null {
	return get(authStore).token;
}
