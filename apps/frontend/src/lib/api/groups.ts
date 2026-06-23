import { apiFetch } from './client';
import type { CreateGroupResponse, Group } from './types';

export function listGroups() {
	return apiFetch<Group[]>('/groups');
}

export function createGroup(name: string) {
	return apiFetch<CreateGroupResponse>('/groups', { method: 'POST', body: JSON.stringify({ name }) });
}

export function updateGroup(id: string, name: string) {
	return apiFetch<void>(`/groups/${id}`, { method: 'PUT', body: JSON.stringify({ name }) });
}

export function deleteGroup(id: string) {
	return apiFetch<void>(`/groups/${id}`, { method: 'DELETE' });
}

export function addGroupMember(groupID: string, userID: string) {
	return apiFetch<{ status: string }>(`/groups/${groupID}/members`, {
		method: 'POST',
		body: JSON.stringify({ user_id: userID })
	});
}

export function removeGroupMember(groupID: string, userID: string) {
	return apiFetch<void>(`/groups/${groupID}/members/${userID}`, { method: 'DELETE' });
}
