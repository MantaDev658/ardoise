<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { APIError } from '$lib/api/client';
	import { deleteExpense, listExpenses } from '$lib/api/expenses';
	import {
		addGroupMember,
		deleteGroup,
		getGroupActivity,
		listGroups,
		removeGroupMember,
		updateGroup
	} from '$lib/api/groups';
	import type { AuditLog, ExpenseItem, Group, User } from '$lib/api/types';
	import { listUsers } from '$lib/api/users';
	import Button from '$lib/components/Button.svelte';
	import HRule from '$lib/components/HRule.svelte';
	import Input from '$lib/components/Input.svelte';
	import Select from '$lib/components/Select.svelte';
	import Window from '$lib/components/Window.svelte';
	import { authStore } from '$lib/stores/auth';
	import { toastStore } from '$lib/stores/toast';
	import { formatCents, formatDate } from '$lib/utils';

	type Tab = 'members' | 'expenses' | 'activity';

	// ── Route ────────────────────────────────────────────────────────
	const groupID = $derived($page.params.id ?? '');

	// ── Data ─────────────────────────────────────────────────────────
	let group = $state<Group | null>(null);
	let allUsers = $state<User[]>([]);
	let expenses = $state<ExpenseItem[]>([]);
	let activity = $state<AuditLog[]>([]);
	let activityCursor = $state('');
	let activityCursorStack = $state<string[]>([]);

	// ── UI ───────────────────────────────────────────────────────────
	let loading = $state(true);
	let tab = $state<Tab>('members');
	let addMemberID = $state('');
	let addingMember = $state(false);

	// ── Rename ───────────────────────────────────────────────────────
	let renamingGroup = $state(false);
	let newGroupName = $state('');
	let renaming = $state(false);

	// ── Delete ───────────────────────────────────────────────────────
	let deletingGroup = $state(false);

	// ── Leave ────────────────────────────────────────────────────────
	let leavingGroup = $state(false);

	// ── Expense pagination ────────────────────────────────────────────
	let expenseNextCursor = $state('');
	let expenseCursorStack = $state<string[]>([]);

	// ── Derived ──────────────────────────────────────────────────────
	const userByID = $derived(Object.fromEntries((allUsers ?? []).map((u) => [u.ID, u])));

	const nonMembers = $derived(
		(allUsers ?? []).filter((u) => !group?.Members.includes(u.ID))
			.map((u) => ({ value: u.ID, label: u.DisplayName }))
	);

	// ── Load ─────────────────────────────────────────────────────────
	let mounted = true;
	onDestroy(() => { mounted = false; });

	onMount(() => {
		loadGroup();
	});

	async function loadGroup() {
		loading = true;
		try {
			const id = groupID;
			const [groups, users] = await Promise.all([listGroups(), listUsers()]);
			if (!mounted) return;
			group = groups.find((g) => g.ID === id) ?? null;
			allUsers = users;
		} catch {
			if (mounted) toastStore.error('Failed to load group.');
		} finally {
			if (mounted) loading = false;
		}
	}

	$effect(() => {
		if (tab === 'expenses' && groupID) {
			expenseCursorStack = [];
			expenseNextCursor = '';
			loadExpenses();
		}
		if (tab === 'activity' && groupID) loadActivity();
	});

	async function loadExpenses(cursor = '') {
		try {
			const result = await listExpenses(groupID, cursor || undefined, 20);
			expenses = result.data;
			expenseNextCursor = result.next_cursor;
		} catch {
			toastStore.error('Failed to load expenses.');
		}
	}

	function expenseNext() {
		expenseCursorStack = [...expenseCursorStack, expenseNextCursor];
		loadExpenses(expenseNextCursor);
	}

	function expensePrev() {
		const stack = [...expenseCursorStack];
		stack.pop();
		const cursor = stack.at(-1) ?? '';
		expenseCursorStack = stack;
		loadExpenses(cursor);
	}

	async function loadActivity(cursor = '') {
		try {
			const result = await getGroupActivity(groupID, cursor || undefined, 20);
			activity = result.data;
			activityCursor = result.next_cursor;
		} catch {
			toastStore.error('Failed to load activity.');
		}
	}

	// ── Members ───────────────────────────────────────────────────────
	async function handleAddMember() {
		if (!addMemberID || addingMember) return;
		addingMember = true;
		try {
			await addGroupMember(groupID, addMemberID);
			toastStore.success('Member added.');
			addMemberID = '';
			const groups = await listGroups();
			group = groups.find((g) => g.ID === groupID) ?? null;
		} catch (err) {
			toastStore.error(err instanceof APIError ? err.message : 'Failed to add member.');
		} finally {
			addingMember = false;
		}
	}

	async function handleRemoveMember(userID: string) {
		if (!confirm(`Remove ${userByID[userID]?.DisplayName ?? userID} from group?`)) return;
		try {
			await removeGroupMember(groupID, userID);
			toastStore.success('Member removed.');
			const groups = await listGroups();
			group = groups.find((g) => g.ID === groupID) ?? null;
		} catch (err) {
			toastStore.error(err instanceof APIError ? err.message : 'Failed to remove member.');
		}
	}

	// ── Rename group ──────────────────────────────────────────────────
	function startRename() {
		newGroupName = group?.Name ?? '';
		renamingGroup = true;
	}

	async function handleRenameGroup() {
		if (!newGroupName.trim() || renaming) return;
		renaming = true;
		try {
			await updateGroup(groupID, newGroupName.trim());
			toastStore.success('Group renamed.');
			renamingGroup = false;
			const groups = await listGroups();
			group = groups.find((g) => g.ID === groupID) ?? null;
		} catch (err) {
			toastStore.error(err instanceof APIError ? err.message : 'Failed to rename group.');
		} finally {
			renaming = false;
		}
	}

	// ── Leave group ───────────────────────────────────────────────────
	async function handleLeaveGroup() {
		if (!confirm(`Leave "${group?.Name}"?`)) return;
		leavingGroup = true;
		try {
			await removeGroupMember(groupID, $authStore.userID!);
			toastStore.success('You have left the group.');
			goto('/groups');
		} catch (err) {
			leavingGroup = false;
			toastStore.error(err instanceof APIError ? err.message : 'Failed to leave group.');
		}
	}

	// ── Delete group ──────────────────────────────────────────────────
	async function handleDeleteGroup() {
		if (!confirm(`Delete "${group?.Name}"? This cannot be undone.`)) return;
		deletingGroup = true;
		try {
			await deleteGroup(groupID);
			toastStore.success('Group deleted.');
			goto('/groups');
		} catch (err) {
			deletingGroup = false;
			toastStore.error(err instanceof APIError ? err.message : 'Failed to delete group.');
		}
	}

	// ── Expenses ──────────────────────────────────────────────────────
	async function handleDeleteExpense(id: string) {
		if (!confirm('Delete this expense?')) return;
		try {
			await deleteExpense(id);
			const cursor = expenseCursorStack.at(-1) ?? '';
			loadExpenses(cursor);
		} catch (err) {
			toastStore.error(err instanceof APIError ? err.message : 'Failed to delete expense.');
		}
	}

	// ── Audit log formatting ──────────────────────────────────────────
	function formatAuditEntry(log: AuditLog): string {
		const actor = userByID[log.user_id]?.DisplayName ?? log.user_id;
		const targetUser = log.target_id ? (userByID[log.target_id]?.DisplayName ?? log.target_id) : null;
		const name = group?.Name ?? 'this group';

		switch (log.action) {
			case 'CREATED_GROUP':
				return `${actor} created "${name}"`;
			case 'ADDED_MEMBER':
				return `${actor} added ${targetUser} to ${name}`;
			case 'REMOVED_GROUP_MEMBER':
				return `${actor} removed ${targetUser} from ${name}`;
			case 'RENAMED_GROUP':
				return `${actor} renamed the group to "${log.details?.replace('Renamed to ', '')}"`;
			case 'DELETED_GROUP':
				return `${actor} deleted the group`;
			case 'CREATED_EXPENSE':
				return `${actor} added expense "${log.details}"`;
			case 'UPDATED_EXPENSE':
				return `${actor} updated expense "${log.details?.replace('Updated: ', '')}"`;
			case 'DELETED_EXPENSE':
				return `${actor} deleted expense "${log.details?.replace('Deleted expense: ', '')}"`;
			case 'SETTLED_DEBT':
				return `${actor} recorded a payment`;
			default:
				return `${actor}: ${log.action}${log.details ? ` — ${log.details}` : ''}`;
		}
	}

	// ── Activity pagination ───────────────────────────────────────────
	function activityNext() {
		activityCursorStack = [...activityCursorStack, activityCursor];
		loadActivity(activityCursor);
	}

	function activityPrev() {
		const stack = [...activityCursorStack];
		stack.pop();
		const cursor = stack.at(-1) ?? '';
		activityCursorStack = stack;
		loadActivity(cursor);
	}
</script>

<svelte:head>
	<title>{group?.Name ?? 'Group'} — Open Split</title>
</svelte:head>

{#if loading}
	<p class="font-system text-white text-sm animate-pulse">Loading…</p>
{:else if !group}
	<p class="font-system text-white text-sm">Group not found or you are not a member.</p>
{:else}
	<Window title={group.Name}>
		<!-- Tab bar -->
		<div class="flex gap-0 -mx-4 -mt-4 mb-4 overflow-x-auto">
			{#each (['members', 'expenses', 'activity'] as Tab[]) as t}
				<button
					class="px-4 py-1.5 text-xs font-bold uppercase font-system shrink-0
					       {tab === t ? 'bg-win95 text-black' : 'bg-win-dark text-white hover:bg-win95 hover:text-black'}"
					style="box-shadow: {tab === t ? 'var(--bevel-out)' : 'var(--bevel-in)'}"
					onclick={() => (tab = t)}
				>
					{t}
				</button>
			{/each}
		</div>

		<!-- Members tab -->
		{#if tab === 'members'}
			<div class="flex flex-col gap-2 font-system text-sm">
				{#each group.Members as memberID, i}
					<div
						class="flex items-center justify-between px-2 py-1
						       {i % 2 === 0 ? 'bg-win-panel' : 'bg-white'}"
					>
						<span>{userByID[memberID]?.DisplayName ?? memberID}</span>
						{#if memberID !== $authStore.userID}
							<Button variant="danger" onclick={() => handleRemoveMember(memberID)}>REMOVE</Button>
						{/if}
					</div>
				{/each}

				<HRule />

				<div class="flex gap-2 mt-1">
					<Select
						bind:value={addMemberID}
						placeholder="Add member…"
						options={nonMembers}
						class="flex-1"
					/>
					<Button variant="success" onclick={handleAddMember} disabled={!addMemberID || addingMember}>
						{addingMember ? '…' : '+ ADD'}
					</Button>
				</div>

				<HRule class="mt-2" />

				<!-- Rename group -->
				{#if renamingGroup}
					<div class="flex gap-2 items-center mt-1">
						<Input bind:value={newGroupName} placeholder="New name…" class="flex-1" />
						<Button variant="success" onclick={handleRenameGroup} disabled={!newGroupName.trim() || renaming}>
							{renaming ? '…' : 'SAVE'}
						</Button>
						<Button onclick={() => (renamingGroup = false)}>CANCEL</Button>
					</div>
				{:else}
					<Button onclick={startRename}>RENAME GROUP</Button>
				{/if}

				<!-- Leave group -->
				<div class="mt-1">
					<Button variant="danger" onclick={handleLeaveGroup} disabled={leavingGroup}>
						{leavingGroup ? 'LEAVING…' : 'LEAVE GROUP'}
					</Button>
				</div>

				<!-- Delete group -->
				<div class="mt-1">
					<Button variant="danger" onclick={handleDeleteGroup} disabled={deletingGroup}>
						{deletingGroup ? 'DELETING…' : 'DELETE GROUP'}
					</Button>
				</div>
			</div>

		<!-- Expenses tab -->
		{:else if tab === 'expenses'}
			{#if !expenses.length}
				<p class="font-system text-sm text-win-dark">No expenses in this group yet.</p>
			{:else}
				<div class="overflow-x-auto">
					<table class="w-full font-system text-sm">
						<thead>
							<tr class="bg-win-navy text-white">
								<th class="px-2 py-1 text-left font-bold">Description</th>
								<th class="px-2 py-1 text-left font-bold">Payer</th>
								<th class="px-2 py-1 text-right font-bold">Amount</th>
								<th class="px-2 py-1 text-right font-bold">Date</th>
								<th class="px-2 py-1"></th>
							</tr>
						</thead>
						<tbody>
							{#each expenses as exp, i}
								<tr class={i % 2 === 0 ? 'bg-win-panel' : 'bg-white'}>
									<td class="px-2 py-0.5">{exp.description}</td>
									<td class="px-2 py-0.5 text-win-dark">{exp.payer}</td>
									<td class="px-2 py-0.5 text-right font-mono">{formatCents(exp.total_cents)}</td>
									<td class="px-2 py-0.5 text-right text-win-dark">{formatDate(exp.created_at)}</td>
									<td class="px-2 py-0.5 text-right">
										<Button variant="danger" onclick={() => handleDeleteExpense(exp.id)}>DEL</Button>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
				<div class="flex gap-2 mt-3">
					<Button onclick={expensePrev} disabled={!expenseCursorStack.length}>◀ PREV</Button>
					<Button onclick={expenseNext} disabled={!expenseNextCursor}>NEXT ▶</Button>
				</div>
			{/if}

		<!-- Activity tab -->
		{:else if tab === 'activity'}
			{#if !activity.length}
				<p class="font-system text-sm text-win-dark">No activity yet.</p>
			{:else}
				<div class="flex flex-col gap-1 font-system text-xs">
					{#each activity as log, i}
						<div class="px-2 py-1 {i % 2 === 0 ? 'bg-win-panel' : 'bg-white'} flex items-baseline gap-3">
							<span class="text-win-dark shrink-0 font-mono">{formatDate(log.created_at)}</span>
							<span>{formatAuditEntry(log)}</span>
						</div>
					{/each}
				</div>
				<div class="flex gap-2 mt-3">
					<Button onclick={activityPrev} disabled={!activityCursorStack.length}>◀ PREV</Button>
					<Button onclick={activityNext} disabled={!activityCursor}>NEXT ▶</Button>
				</div>
			{/if}
		{/if}
	</Window>
{/if}
