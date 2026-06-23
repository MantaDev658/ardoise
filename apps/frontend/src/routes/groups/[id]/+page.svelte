<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { APIError } from '$lib/api/client';
	import { deleteExpense, getBalances, listExpenses } from '$lib/api/expenses';
	import {
		addGroupMember,
		deleteGroup,
		listGroups,
		removeGroupMember,
		updateGroup
	} from '$lib/api/groups';
	import type { BalancesResponse, ExpenseItem, Group, User } from '$lib/api/types';
	import { listUsers } from '$lib/api/users';
	import Button from '$lib/components/Button.svelte';
	import HRule from '$lib/components/HRule.svelte';
	import Input from '$lib/components/Input.svelte';
	import Window from '$lib/components/Window.svelte';
	import { authStore } from '$lib/stores/auth';
	import { toastStore } from '$lib/stores/toast';
	import { formatCents, formatDate } from '$lib/utils';

	type Tab = 'members' | 'expenses' | 'balances';

	// ── Route ────────────────────────────────────────────────────────
	const groupID = $derived($page.params.id ?? '');

	// ── Data ─────────────────────────────────────────────────────────
	let group = $state<Group | null>(null);
	let allUsers = $state<User[]>([]);
	let expenses = $state<ExpenseItem[]>([]);

	// ── UI ───────────────────────────────────────────────────────────
	let loading = $state(true);
	let tab = $state<Tab>('members');
	let addMemberSearch = $state('');
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
		if (tab === 'balances' && groupID) loadBalances();
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

	// ── Members ───────────────────────────────────────────────────────
	async function handleAddMember() {
		const trimmed = addMemberSearch.trim();
		if (!trimmed || addingMember) return;
		addingMember = true;
		try {
			await addGroupMember(groupID, trimmed);
			toastStore.success(`Invitation sent to ${trimmed}.`);
			addMemberSearch = '';
		} catch (err) {
			toastStore.error(err instanceof APIError ? err.message : 'Failed to send invitation.');
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

	// ── Balances ──────────────────────────────────────────────────────
	let balances = $state<BalancesResponse | null>(null);

	async function loadBalances() {
		try {
			balances = await getBalances(groupID);
		} catch {
			toastStore.error('Failed to load balances.');
		}
	}
</script>

<svelte:head>
	<title>{group?.Name ?? 'Group'} — Ardoise</title>
</svelte:head>

{#if loading}
	<p class="font-system text-white text-sm animate-pulse">Loading…</p>
{:else if !group}
	<p class="font-system text-white text-sm">Group not found or you are not a member.</p>
{:else}
	<Window title={group.Name}>
		<!-- Tab bar -->
		<div class="flex gap-0 -mx-2 -mt-2 sm:-mx-4 sm:-mt-4 mb-4 overflow-x-auto">
			{#each (['members', 'balances', 'expenses'] as Tab[]) as t}
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
					<Input
						bind:value={addMemberSearch}
						placeholder="Enter username…"
						class="flex-1"
					/>
					<Button variant="success" onclick={handleAddMember} disabled={!addMemberSearch.trim() || addingMember}>
						{addingMember ? '…' : '+ ADD'}
					</Button>
				</div>

				<HRule class="mt-2" />

				<!-- Group actions -->
				{#if renamingGroup}
					<div class="flex gap-2 items-center mt-1">
						<Input bind:value={newGroupName} placeholder="New name…" class="flex-1" />
						<Button variant="success" onclick={handleRenameGroup} disabled={!newGroupName.trim() || renaming}>
							{renaming ? '…' : 'SAVE'}
						</Button>
						<Button onclick={() => (renamingGroup = false)}>CANCEL</Button>
					</div>
				{:else}
					<div class="flex gap-2 mt-1">
						<Button class="flex-1" onclick={startRename}>RENAME</Button>
						<Button class="flex-1" variant="danger" onclick={handleLeaveGroup} disabled={leavingGroup}>
							{leavingGroup ? 'LEAVING…' : 'LEAVE'}
						</Button>
						<Button class="flex-1" variant="danger" onclick={handleDeleteGroup} disabled={deletingGroup}>
							{deletingGroup ? 'DELETING…' : 'DELETE'}
						</Button>
					</div>
				{/if}
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
								<th class="px-2 py-1 text-right font-bold hidden sm:table-cell">Date</th>
								<th class="px-2 py-1"></th>
							</tr>
						</thead>
						<tbody>
							{#each expenses as exp, i}
								<tr class={i % 2 === 0 ? 'bg-win-panel' : 'bg-white'}>
									<td class="px-2 py-0.5 max-w-[40vw] sm:max-w-none truncate">{exp.description}</td>
									<td class="px-2 py-0.5 text-win-dark max-w-[24vw] sm:max-w-none truncate">{exp.payer}</td>
									<td class="px-2 py-0.5 text-right font-mono whitespace-nowrap">{formatCents(exp.total_cents)}</td>
									<td class="px-2 py-0.5 text-right text-win-dark hidden sm:table-cell">{formatDate(exp.created_at)}</td>
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

		<!-- Balances tab -->
		{:else if tab === 'balances'}
			{#if !balances}
				<p class="font-system text-sm animate-pulse">Loading…</p>
			{:else}
				<!-- Net balances -->
				<p class="font-system text-xs font-bold uppercase mb-1">Net Balances</p>
				{#if Object.keys(balances.net_balances).length === 0}
					<p class="font-system text-sm text-win-dark">No balances — group is settled up.</p>
				{:else}
					<table class="w-full font-system text-sm mb-4">
						<tbody>
							{#each Object.entries(balances.net_balances) as [uid, cents], i}
								<tr class={i % 2 === 0 ? 'bg-win-panel' : 'bg-white'}>
									<td class="px-2 py-1 max-w-0 w-3/5 truncate">{userByID[uid]?.DisplayName ?? uid}</td>
									<td class="px-2 py-1 text-right font-mono font-bold whitespace-nowrap
										{cents > 0 ? 'text-green-700' : cents < 0 ? 'text-red-700' : 'text-win-dark'}">
										{cents > 0 ? '+' : ''}{formatCents(cents)}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}

				<!-- Suggested transfers -->
				<p class="font-system text-xs font-bold uppercase mb-1">Suggested Transfers</p>
				{#if !balances.suggested_settlements?.length}
					<p class="font-system text-sm text-win-dark">Nothing to settle — everyone is even.</p>
				{:else}
					<div class="flex flex-col gap-1">
						{#each balances.suggested_settlements as s, i}
							<div class="flex items-center justify-between px-2 py-1
								{i % 2 === 0 ? 'bg-win-panel' : 'bg-white'}">
								<span class="font-system text-sm min-w-0 truncate">
									{userByID[s.From]?.DisplayName ?? s.From}
									→
									{userByID[s.To]?.DisplayName ?? s.To}
									<span class="font-mono font-bold ml-2 whitespace-nowrap">{formatCents(s.Amount)}</span>
								</span>
								<a
									href="/settle?to={s.To}&amount={s.Amount}&group={groupID}"
									class="inline-block shrink-0 ml-2"
								>
									<Button variant="success">SETTLE</Button>
								</a>
							</div>
						{/each}
					</div>
				{/if}
			{/if}
		{/if}
	</Window>
{/if}
