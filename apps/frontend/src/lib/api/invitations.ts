import { apiFetch } from './client';
import type { Invitation } from './types';

export function listMyInvitations() {
	return apiFetch<Invitation[]>('/invitations');
}

export function acceptInvitation(id: string) {
	return apiFetch<void>(`/invitations/${id}/accept`, { method: 'POST' });
}

export function declineInvitation(id: string) {
	return apiFetch<void>(`/invitations/${id}/decline`, { method: 'POST' });
}
