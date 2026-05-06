<script lang="ts">
	import { APIError } from '$lib/api/client';
	import { getBalances } from '$lib/api/expenses';
	import { createGroup, listGroups } from '$lib/api/groups';
	import type { Group } from '$lib/api/types';
	import Button from '$lib/components/Button.svelte';
	import Input from '$lib/components/Input.svelte';
	import Window from '$lib/components/Window.svelte';
	import { toastStore } from '$lib/stores/toast';

	type GroupStatus = { settled: boolean; count: number };

	let groups = $state<Group[]>([]);
	let groupStatuses = $state<Record<string, GroupStatus>>({});
	let loading = $state(true);
	let showForm = $state(false);
	let newName = $state('');
	let submitting = $state(false);

	$effect(() => {
		loadAll();
	});

	async function loadAll() {
		loading = true;
		try {
			const g = await listGroups();
			groups = g;
			const entries = await Promise.all(
				g.map(async (grp) => {
					try {
						const bal = await getBalances(grp.ID);
						const count = bal.suggested_settlements?.length ?? 0;
						return [grp.ID, { settled: count === 0, count }] as const;
					} catch {
						return [grp.ID, { settled: true, count: 0 }] as const;
					}
				})
			);
			groupStatuses = Object.fromEntries(entries);
		} catch {
			toastStore.error('Failed to load groups.');
		} finally {
			loading = false;
		}
	}

	async function handleCreate(e: SubmitEvent) {
		e.preventDefault();
		if (!newName.trim() || submitting) return;
		submitting = true;
		try {
			await createGroup(newName.trim());
			toastStore.success('Group created!');
			newName = '';
			showForm = false;
			await loadAll();
		} catch (err) {
			toastStore.error(err instanceof APIError ? err.message : 'Failed to create group.');
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>Groups — Open Split</title>
</svelte:head>

<Window title="MY GROUPS">
	<div class="flex items-center gap-2 mb-4">
		<Button
			variant={showForm ? 'default' : 'primary'}
			onclick={() => {
				showForm = !showForm;
				newName = '';
			}}
		>
			{showForm ? 'CANCEL' : '+ CREATE GROUP'}
		</Button>
	</div>

	{#if showForm}
		<form class="flex gap-2 mb-4 font-system" onsubmit={handleCreate}>
			<Input bind:value={newName} placeholder="Group name…" class="flex-1" />
			<Button type="submit" variant="success" disabled={submitting || !newName.trim()}>
				{submitting ? 'SAVING…' : 'CREATE'}
			</Button>
		</form>
	{/if}

	{#if loading}
		<p class="font-system text-sm text-win-dark animate-pulse">Loading…</p>
	{:else if !groups.length}
		<p class="font-system text-sm text-win-dark">No groups yet. Create one above.</p>
	{:else}
		<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
			{#each groups as group}
				{@const status = groupStatuses[group.ID]}
				<a href="/groups/{group.ID}" class="block no-underline">
					<div class="bg-win95 h-full" style="box-shadow: var(--bevel-out-deep); padding: 2px">
						<div
							class="px-2 py-1 font-system font-bold text-sm text-white truncate select-none"
							style="background: linear-gradient(to right, #000080, #1084d0)"
						>
							{group.Name}
						</div>
						<div class="bg-white p-3" style="box-shadow: var(--bevel-in)">
							<p class="font-system text-sm text-win-dark">
								{group.Members.length} member{group.Members.length === 1 ? '' : 's'}
							</p>
							{#if status === undefined}
								<p class="font-system text-xs text-win-dark mt-1 animate-pulse">Checking…</p>
							{:else if status.settled}
								<p class="font-system text-xs font-bold mt-1" style="color: #008000">✓ Settled up</p>
							{:else}
								<p class="font-system text-xs font-bold mt-1 text-win-red">
									⚠ {status.count} payment{status.count === 1 ? '' : 's'} needed
								</p>
							{/if}
							<p class="font-system text-xs text-win-accent mt-1">Open →</p>
						</div>
					</div>
				</a>
			{/each}
		</div>
	{/if}
</Window>
