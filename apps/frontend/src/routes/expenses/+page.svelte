<script lang="ts">
	import { APIError } from '$lib/api/client';
	import { createExpense, deleteExpense, listExpenses, updateExpense } from '$lib/api/expenses';
	import { listGroups } from '$lib/api/groups';
	import type { ExpenseItem, Group, SplitInput, SplitType, User } from '$lib/api/types';
	import { listUsers } from '$lib/api/users';
	import Button from '$lib/components/Button.svelte';
	import HRule from '$lib/components/HRule.svelte';
	import Input from '$lib/components/Input.svelte';
	import Select from '$lib/components/Select.svelte';
	import Window from '$lib/components/Window.svelte';
	import { formatCents, formatDate } from '$lib/utils';

	// ── Data ────────────────────────────────────────────────────────
	let expenses = $state<ExpenseItem[]>([]);
	let groups = $state<Group[]>([]);
	let users = $state<User[]>([]);
	let nextCursor = $state('');
	let cursorStack = $state<string[]>([]);

	// ── UI state ────────────────────────────────────────────────────
	let loading = $state(true);
	let error = $state('');
	let groupFilter = $state('');
	let showForm = $state(false);

	// ── Create form ─────────────────────────────────────────────────
	let desc = $state('');
	let totalDollars = $state('');
	let splitType = $state<SplitType>('EQUAL');
	let formGroupId = $state('');
	let participants = $state<{ userID: string; value: string }[]>([]);
	let addUserID = $state('');
	let formError = $state('');
	let submitting = $state(false);

	// ── Edit form ────────────────────────────────────────────────────
	let editingID = $state('');
	let editDesc = $state('');
	let editDollars = $state('');
	let editSplitType = $state<SplitType>('EXACT');
	let editGroupId = $state('');
	let editPayer = $state('');
	let editParticipants = $state<{ userID: string; value: string }[]>([]);
	let editAddUserID = $state('');
	let editError = $state('');
	let editSaving = $state(false);

	// ── Derived ─────────────────────────────────────────────────────
	const userByID = $derived(Object.fromEntries(users.map((u) => [u.ID, u])));

	const groupOptions = $derived([
		{ value: '', label: 'All Expenses' },
		...groups.map((g) => ({ value: g.ID, label: g.Name }))
	]);

	const userOptions = $derived(
		users
			.filter((u) => !participants.some((p) => p.userID === u.ID))
			.map((u) => ({ value: u.ID, label: u.DisplayName }))
	);

	const splitValueMeta = $derived(
		splitType === 'EXACT'
			? { label: 'Amount ($)', placeholder: '0.00' }
			: splitType === 'PERCENTAGE'
				? { label: 'Percent (%)', placeholder: '0' }
				: splitType === 'SHARES'
					? { label: 'Shares', placeholder: '1' }
					: null
	);

	const editSplitValueMeta = $derived(
		editSplitType === 'EXACT'
			? { label: 'Amount ($)', placeholder: '0.00' }
			: editSplitType === 'PERCENTAGE'
				? { label: 'Percent (%)', placeholder: '0' }
				: editSplitType === 'SHARES'
					? { label: 'Shares', placeholder: '1' }
					: null
	);

	const editUserOptions = $derived(
		users
			.filter((u) => !editParticipants.some((p) => p.userID === u.ID))
			.map((u) => ({ value: u.ID, label: u.DisplayName }))
	);

	// ── Auto-populate participants when group is selected ─────────────
	$effect(() => {
		const gid = formGroupId;
		if (gid) {
			const g = groups.find((grp) => grp.ID === gid);
			if (g) {
				participants = g.Members.map((memberID) => ({ userID: memberID, value: '' }));
			}
		} else {
			participants = [];
		}
	});

	// ── Data loading ─────────────────────────────────────────────────
	$effect(() => {
		loadAll();
	});

	async function loadAll() {
		loading = true;
		error = '';
		try {
			const [exp, grps, usrs] = await Promise.all([
				listExpenses(undefined, undefined, 20),
				listGroups(),
				listUsers()
			]);
			expenses = exp.data;
			nextCursor = exp.next_cursor;
			groups = grps;
			users = usrs;
		} catch {
			error = 'Failed to load expenses.';
		} finally {
			loading = false;
		}
	}

	async function loadPage(cursor = '') {
		loading = true;
		error = '';
		try {
			const result = await listExpenses(groupFilter || undefined, cursor || undefined, 20);
			expenses = result.data;
			nextCursor = result.next_cursor;
		} catch {
			error = 'Failed to load expenses.';
		} finally {
			loading = false;
		}
	}

	function applyFilter() {
		cursorStack = [];
		loadPage();
	}

	function goNext() {
		cursorStack = [...cursorStack, nextCursor];
		loadPage(nextCursor);
	}

	function goPrev() {
		const stack = [...cursorStack];
		stack.pop();
		const cursor = stack.at(-1) ?? '';
		cursorStack = stack;
		loadPage(cursor);
	}

	// ── Create expense ───────────────────────────────────────────────
	function addParticipant() {
		if (!addUserID) return;
		participants = [...participants, { userID: addUserID, value: '' }];
		addUserID = '';
	}

	function removeParticipant(userID: string) {
		participants = participants.filter((p) => p.userID !== userID);
	}

	function updateParticipantValue(userID: string, value: string) {
		participants = participants.map((p) => (p.userID === userID ? { ...p, value } : p));
	}

	function resetForm() {
		desc = '';
		totalDollars = '';
		splitType = 'EQUAL';
		formGroupId = '';
		participants = [];
		addUserID = '';
		formError = '';
	}

	async function handleCreate(e: SubmitEvent) {
		e.preventDefault();
		if (submitting) return;

		const totalCents = Math.round(parseFloat(totalDollars) * 100);
		if (!desc.trim()) { formError = 'Description is required.'; return; }
		if (isNaN(totalCents) || totalCents <= 0) { formError = 'Enter a valid amount.'; return; }
		if (participants.length === 0) { formError = 'Add at least one participant.'; return; }

		const splits: SplitInput[] = participants.map((p) => {
			if (splitType === 'EQUAL') return { user_id: p.userID };
			const num = parseFloat(p.value);
			const value = splitType === 'EXACT' ? Math.round(num * 100) : Math.round(num);
			return { user_id: p.userID, value };
		});

		submitting = true;
		formError = '';
		try {
			await createExpense({
				description: desc.trim(),
				total_cents: totalCents,
				split_type: splitType,
				splits,
				...(formGroupId ? { group_id: formGroupId } : {})
			});
			resetForm();
			showForm = false;
			cursorStack = [];
			loadPage();
		} catch (err) {
			formError = err instanceof APIError ? err.message : 'Failed to create expense.';
		} finally {
			submitting = false;
		}
	}

	// ── Edit expense ──────────────────────────────────────────────────
	function startEdit(exp: ExpenseItem) {
		editingID = exp.id;
		editDesc = exp.description;
		editDollars = String(exp.total_cents / 100);
		editSplitType = 'EXACT';
		editGroupId = exp.group_id ?? '';
		editPayer = exp.payer;
		editParticipants = exp.splits.map((s) => ({
			userID: s.user_id,
			value: String(s.amount_cents / 100)
		}));
		editAddUserID = '';
		editError = '';
	}

	function cancelEdit() {
		editingID = '';
		editError = '';
	}

	function addEditParticipant() {
		if (!editAddUserID) return;
		editParticipants = [...editParticipants, { userID: editAddUserID, value: '' }];
		editAddUserID = '';
	}

	function removeEditParticipant(userID: string) {
		editParticipants = editParticipants.filter((p) => p.userID !== userID);
	}

	function updateEditParticipantValue(userID: string, value: string) {
		editParticipants = editParticipants.map((p) => (p.userID === userID ? { ...p, value } : p));
	}

	async function handleEdit(e: SubmitEvent) {
		e.preventDefault();
		if (editSaving) return;

		const totalCents = Math.round(parseFloat(editDollars) * 100);
		if (!editDesc.trim()) { editError = 'Description is required.'; return; }
		if (isNaN(totalCents) || totalCents <= 0) { editError = 'Enter a valid amount.'; return; }
		if (editParticipants.length === 0) { editError = 'Add at least one participant.'; return; }

		const splits: SplitInput[] = editParticipants.map((p) => {
			if (editSplitType === 'EQUAL') return { user_id: p.userID };
			const num = parseFloat(p.value);
			const value = editSplitType === 'EXACT' ? Math.round(num * 100) : Math.round(num);
			return { user_id: p.userID, value };
		});

		editSaving = true;
		editError = '';
		try {
			await updateExpense(editingID, {
				description: editDesc.trim(),
				total_cents: totalCents,
				payer: editPayer,
				split_type: editSplitType,
				splits,
				...(editGroupId ? { group_id: editGroupId } : {})
			});
			cancelEdit();
			const cursor = cursorStack.at(-1) ?? '';
			loadPage(cursor);
		} catch (err) {
			editError = err instanceof APIError ? err.message : 'Failed to update expense.';
		} finally {
			editSaving = false;
		}
	}

	// ── Delete expense ────────────────────────────────────────────────
	async function handleDelete(id: string) {
		if (!confirm('Delete this expense?')) return;
		try {
			await deleteExpense(id);
			const cursor = cursorStack.at(-1) ?? '';
			loadPage(cursor);
		} catch (err) {
			error = err instanceof APIError ? err.message : 'Failed to delete expense.';
		}
	}
</script>

<svelte:head>
	<title>Expenses — Open Split</title>
</svelte:head>

<Window title="EXPENSE LEDGER">
	<!-- Toolbar -->
	<div class="flex items-center gap-2 mb-3 flex-wrap">
		<Button
			onclick={() => {
				showForm = !showForm;
				if (!showForm) resetForm();
			}}
			variant={showForm ? 'default' : 'primary'}
		>
			{showForm ? 'CANCEL' : '+ ADD EXPENSE'}
		</Button>

		<div class="flex items-center gap-1 ml-auto">
			<label class="text-xs font-system font-bold shrink-0" for="group-filter">GROUP:</label>
			<Select
				id="group-filter"
				bind:value={groupFilter}
				options={groupOptions}
				class="w-40"
				onchange={applyFilter}
			/>
		</div>
	</div>

	<!-- Create form -->
	{#if showForm}
		<div class="mb-4">
			<HRule />
			<form class="mt-3 flex flex-col gap-3 font-system" onsubmit={handleCreate}>
				<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
					<div class="flex flex-col gap-1">
						<label class="text-xs font-bold" for="exp-desc">Description</label>
						<Input id="exp-desc" bind:value={desc} placeholder="Dinner, Hotel, etc." />
					</div>
					<div class="flex flex-col gap-1">
						<label class="text-xs font-bold" for="exp-total">Total ($)</label>
						<Input id="exp-total" type="number" min="0.01" step="0.01" bind:value={totalDollars} placeholder="0.00" />
					</div>
					<div class="flex flex-col gap-1">
						<label class="text-xs font-bold" for="exp-split">Split Type</label>
						<Select
							id="exp-split"
							bind:value={splitType}
							options={[
								{ value: 'EQUAL', label: 'Even' },
								{ value: 'EXACT', label: 'Exact amounts' },
								{ value: 'PERCENTAGE', label: 'Percentages' },
								{ value: 'SHARES', label: 'Shares' }
							]}
						/>
					</div>
					<div class="flex flex-col gap-1">
						<label class="text-xs font-bold" for="exp-group">Group (optional)</label>
						<Select
							id="exp-group"
							bind:value={formGroupId}
							placeholder="No group"
							options={groups.map((g) => ({ value: g.ID, label: g.Name }))}
						/>
					</div>
				</div>

				<!-- Participants -->
				<div class="flex flex-col gap-1">
					<span class="text-xs font-bold">Participants</span>
					{#if formGroupId}
						<p class="text-xs text-win-dark italic">All group members included — remove as needed</p>
					{/if}
					{#each participants as p}
						{#if splitValueMeta}
							<div class="flex flex-col gap-0.5 mb-0.5">
								<span class="text-xs font-bold text-win-dark">
									{userByID[p.userID]?.DisplayName ?? p.userID}
								</span>
								<div class="flex items-center gap-2">
									<Input
										type="number"
										min="0"
										step={splitType === 'EXACT' ? '0.01' : '1'}
										placeholder={splitValueMeta.placeholder}
										value={p.value}
										oninput={(e: Event) => updateParticipantValue(p.userID, (e.currentTarget as HTMLInputElement).value)}
										class="w-28"
									/>
									<span class="text-xs text-win-dark shrink-0">{splitValueMeta.label}</span>
									<Button variant="danger" onclick={() => removeParticipant(p.userID)}>✕</Button>
								</div>
							</div>
						{:else}
							<div class="flex items-center justify-between gap-2">
								<span class="text-xs truncate">{userByID[p.userID]?.DisplayName ?? p.userID}</span>
								<Button variant="danger" onclick={() => removeParticipant(p.userID)}>✕</Button>
							</div>
						{/if}
					{/each}
					{#if !formGroupId}
						<div class="flex gap-2 mt-1">
							<Select
								bind:value={addUserID}
								placeholder="Select user…"
								options={userOptions}
								class="flex-1"
							/>
							<Button onclick={addParticipant} disabled={!addUserID}>+ ADD</Button>
						</div>
					{/if}
				</div>

				{#if formError}
					<p class="text-xs text-win-red px-2 py-1" style="box-shadow: var(--bevel-in)">
						⚠ {formError}
					</p>
				{/if}

				<div class="flex gap-2">
					<Button type="submit" variant="success" disabled={submitting}>
						{submitting ? 'SAVING…' : 'SAVE EXPENSE'}
					</Button>
				</div>
			</form>
			<HRule class="mt-3" />
		</div>
	{/if}

	<!-- Error -->
	{#if error}
		<p class="text-xs text-win-red mb-2 font-system">⚠ {error}</p>
	{/if}

	<!-- Table -->
	{#if loading}
		<p class="font-system text-sm text-win-dark animate-pulse">Loading…</p>
	{:else if !expenses.length}
		<p class="font-system text-sm text-win-dark">No expenses found.</p>
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
						{#if editingID === exp.id}
							<tr class={i % 2 === 0 ? 'bg-win-panel' : 'bg-white'}>
								<td colspan="5" class="px-2 py-2">
									<form class="flex flex-col gap-2 font-system text-xs" onsubmit={handleEdit}>
										<div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
											<div class="flex flex-col gap-1 col-span-2">
												<label class="font-bold" for="edit-desc">Description</label>
												<Input id="edit-desc" bind:value={editDesc} placeholder="Dinner, Hotel, etc." />
											</div>
											<div class="flex flex-col gap-1">
												<label class="font-bold" for="edit-total">Total ($)</label>
												<Input id="edit-total" type="number" min="0.01" step="0.01" bind:value={editDollars} placeholder="0.00" />
											</div>
											<div class="flex flex-col gap-1">
												<label class="font-bold" for="edit-split">Split Type</label>
												<Select
													id="edit-split"
													bind:value={editSplitType}
													options={[
														{ value: 'EQUAL', label: 'Even' },
														{ value: 'EXACT', label: 'Exact amounts' },
														{ value: 'PERCENTAGE', label: 'Percentages' },
														{ value: 'SHARES', label: 'Shares' }
													]}
												/>
											</div>
										</div>

										<!-- Edit participants -->
										<div class="flex flex-col gap-1">
											<span class="font-bold">Participants</span>
											{#each editParticipants as p}
												{#if editSplitValueMeta}
													<div class="flex flex-col gap-0.5 mb-0.5">
														<span class="font-bold text-win-dark">{userByID[p.userID]?.DisplayName ?? p.userID}</span>
														<div class="flex items-center gap-2">
															<Input
																type="number"
																min="0"
																step={editSplitType === 'EXACT' ? '0.01' : '1'}
																placeholder={editSplitValueMeta.placeholder}
																value={p.value}
																oninput={(e: Event) => updateEditParticipantValue(p.userID, (e.currentTarget as HTMLInputElement).value)}
																class="w-28"
															/>
															<span class="text-win-dark shrink-0">{editSplitValueMeta.label}</span>
															<Button variant="danger" onclick={() => removeEditParticipant(p.userID)}>✕</Button>
														</div>
													</div>
												{:else}
													<div class="flex items-center justify-between gap-2">
														<span class="truncate">{userByID[p.userID]?.DisplayName ?? p.userID}</span>
														<Button variant="danger" onclick={() => removeEditParticipant(p.userID)}>✕</Button>
													</div>
												{/if}
											{/each}
											{#if !editGroupId}
												<div class="flex gap-2 mt-1">
													<Select
														bind:value={editAddUserID}
														placeholder="Add user…"
														options={editUserOptions}
														class="flex-1"
													/>
													<Button onclick={addEditParticipant} disabled={!editAddUserID}>+ ADD</Button>
												</div>
											{/if}
										</div>

										{#if editError}
											<p class="text-win-red px-2 py-1" style="box-shadow: var(--bevel-in)">⚠ {editError}</p>
										{/if}

										<div class="flex gap-2">
											<Button type="submit" variant="success" disabled={editSaving}>
												{editSaving ? 'SAVING…' : 'SAVE'}
											</Button>
											<Button onclick={cancelEdit}>CANCEL</Button>
										</div>
									</form>
								</td>
							</tr>
						{:else}
							<tr class={i % 2 === 0 ? 'bg-win-panel' : 'bg-white'}>
								<td class="px-2 py-0.5">{exp.description}</td>
								<td class="px-2 py-0.5 text-win-dark">{exp.payer}</td>
								<td class="px-2 py-0.5 text-right font-mono">{formatCents(exp.total_cents)}</td>
								<td class="px-2 py-0.5 text-right text-win-dark">{formatDate(exp.created_at)}</td>
								<td class="px-2 py-0.5 text-right">
									<div class="flex gap-1 justify-end">
										<Button onclick={() => startEdit(exp)}>EDIT</Button>
										<Button variant="danger" onclick={() => handleDelete(exp.id)}>DEL</Button>
									</div>
								</td>
							</tr>
						{/if}
					{/each}
				</tbody>
			</table>
		</div>

		<!-- Pagination -->
		<div class="flex items-center gap-2 mt-3 font-system text-sm">
			<Button onclick={goPrev} disabled={cursorStack.length === 0}>◀ PREV</Button>
			<Button onclick={goNext} disabled={!nextCursor}>NEXT ▶</Button>
		</div>
	{/if}
</Window>
