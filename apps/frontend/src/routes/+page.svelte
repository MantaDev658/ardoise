<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { getBalances, listExpenses } from '$lib/api/expenses';
	import { listGroups } from '$lib/api/groups';
	import { listUsers } from '$lib/api/users';
	import HitCounter from '$lib/components/HitCounter.svelte';
	import Window from '$lib/components/Window.svelte';
	import { authStore } from '$lib/stores/auth';
	import { formatCents, formatDate } from '$lib/utils';
	import type { BalancesResponse, ExpenseItem, Group, User } from '$lib/api/types';

	let globalBalances = $state<BalancesResponse | null>(null);
	let groups = $state<Group[]>([]);
	let users = $state<User[]>([]);
	let expenses = $state<ExpenseItem[]>([]);
	let groupStatuses = $state<Record<string, { settled: boolean; count: number }>>({});
	let loading = $state(true);
	let unavailable = $state(false);

	// Guard against setting state on a destroyed component (async loadDashboard
	// can outlive the component when the user navigates away mid-load).
	let mounted = true;
	onDestroy(() => { mounted = false; });

	const userID = $derived($authStore.userID ?? '');
	const userBalance = $derived(globalBalances?.net_balances?.[userID] ?? 0);
	const owedToMe = $derived(Math.max(0, userBalance));
	const iOwe = $derived(Math.max(0, -userBalance));
	const userByID = $derived(Object.fromEntries((users ?? []).map((u) => [u.ID, u])));

	onMount(() => {
		loadDashboard();
	});

	async function loadDashboard() {
		loading = true;
		unavailable = false;

		try {
			const [balResult, grpResult, usrResult, expResult] = await Promise.allSettled([
				getBalances(),
				listGroups(),
				listUsers(),
				listExpenses(undefined, undefined, 5)
			]);

			if (!mounted) return;

			// If every call failed the backend is likely not running yet
			if ([balResult, grpResult, usrResult, expResult].every((r) => r.status === 'rejected')) {
				unavailable = true;
				return;
			}

			globalBalances = balResult.status === 'fulfilled' ? balResult.value : null;
			groups = grpResult.status === 'fulfilled' ? grpResult.value : [];
			users = usrResult.status === 'fulfilled' ? usrResult.value : [];
			expenses = expResult.status === 'fulfilled' ? (expResult.value?.data ?? []) : [];

			// Per-group balance status — allSettled so a slow/failed group call never blocks loading
			const statusResults = await Promise.allSettled(
				(groups ?? []).map((g) => getBalances(g.ID))
			);

			if (!mounted) return;

			groupStatuses = Object.fromEntries(
				(groups ?? []).map((g, i) => {
					const r = statusResults[i];
					if (r?.status === 'fulfilled') {
						const count = r.value.suggested_settlements?.length ?? 0;
						return [g.ID, { settled: count === 0, count }];
					}
					return [g.ID, { settled: true, count: 0 }];
				})
			);
		} finally {
			if (mounted) loading = false;
		}
	}
</script>

<svelte:head>
	<title>Dashboard — Ardoise</title>
</svelte:head>

{#if loading}
	<p class="font-system text-white text-sm animate-pulse">Loading…</p>
{:else if unavailable}
	<div class="font-system text-sm flex flex-col gap-3 text-white">
		<p class="font-bold">⚠ Could not reach the server.</p>
		<p class="text-win-dark text-xs">Make sure the backend is running, then</p>
		<button
			class="text-xs underline text-win-accent text-left"
			onclick={() => loadDashboard()}
		>
			retry →
		</button>
	</div>
{:else}
	{#if !groups?.length && !expenses?.length}
		<!-- Getting started — no data yet -->
		<Window title="WELCOME TO ARDOISE">
			<div class="font-system text-sm flex flex-col gap-3">
				<p class="font-bold">You're all set up! Here's how to get started:</p>
				<ol class="list-decimal list-inside flex flex-col gap-2 text-win-dark">
					<li>
						<a href="/groups" class="text-win-accent underline font-bold">Create a group</a>
						— Ski trip, apartment, road trip, etc.
					</li>
					<li>Add your friends as group members</li>
					<li>
						<a href="/expenses" class="text-win-accent underline font-bold">Log an expense</a>
						— who paid, how to split it
					</li>
					<li>When it's time to settle,
						<a href="/settle" class="text-win-accent underline font-bold">record a payment</a>
					</li>
				</ol>
			</div>
		</Window>
	{:else}
	<!-- Balance totals -->
	<div class="flex gap-4 mb-4 flex-wrap">
		<HitCounter label="You Are Owed" value={owedToMe} />
		<HitCounter label="You Owe" value={iOwe} />
	</div>

	<!-- Groups + Settlements grid -->
	<div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
		<!-- Groups with settled status -->
		<Window title="MY GROUPS">
			{#if !groups?.length}
				<p class="font-system text-sm text-win-dark">
					No groups yet.
					<a href="/groups" class="text-win-accent underline">Create one →</a>
				</p>
			{:else}
				<div class="flex flex-col gap-1 font-system text-sm">
					{#each (groups ?? []) as g, i}
						{@const status = groupStatuses[g.ID]}
						<a href="/groups/{g.ID}" class="block no-underline">
							<div class="flex items-center justify-between px-2 py-1.5
							            {i % 2 === 0 ? 'bg-win-panel' : 'bg-white'}">
								<span class="font-bold truncate">{g.Name}</span>
								<span class="shrink-0 ml-3 text-xs font-bold">
									{#if status === undefined}
										<span class="text-win-dark animate-pulse">…</span>
									{:else if status.settled}
										<span style="color: #008000">✓ SETTLED</span>
									{:else}
										<span class="text-win-red">⚠ {status.count} unsettled</span>
									{/if}
								</span>
							</div>
						</a>
					{/each}
				</div>
				<a href="/groups" class="block text-xs text-win-accent underline mt-2 font-system">
					Manage groups →
				</a>
			{/if}
		</Window>

		<!-- Suggested settlements (global) -->
		<Window title="SUGGESTED SETTLEMENTS">
			{#if !globalBalances?.suggested_settlements?.length}
				<p class="font-system text-sm text-win-dark">All settled up! ✓</p>
			{:else}
				<table class="w-full font-system text-sm">
					<tbody>
						{#each globalBalances.suggested_settlements as s, i}
							<tr class="{i % 2 === 0 ? 'bg-win-panel' : 'bg-white'} leading-6">
								<td class="px-2 font-bold truncate max-w-0 w-2/5">
									{userByID[s.From]?.DisplayName ?? s.From}
								</td>
								<td class="px-1 text-win-dark">→</td>
								<td class="px-2 truncate max-w-0 w-2/5">
									{userByID[s.To]?.DisplayName ?? s.To}
								</td>
								<td class="px-2 text-right font-mono whitespace-nowrap">
									{formatCents(s.Amount)}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
				<a href="/settle" class="block text-xs text-win-accent underline mt-2 font-system">
					Record a payment →
				</a>
			{/if}
		</Window>
	</div>

	<!-- Construction stripe divider -->
	<div class="bg-construction h-6 mb-4" aria-hidden="true"></div>

	<!-- Recent expenses -->
	<Window title="RECENT EXPENSES">
		{#if !expenses?.length}
			<p class="font-system text-sm text-win-dark">
				No expenses yet.
				<a href="/expenses" class="text-win-accent underline">Add one →</a>
			</p>
		{:else}
			<table class="w-full font-system text-sm">
				<thead>
					<tr class="bg-win-navy text-white">
						<th class="px-2 py-1 text-left font-bold">Description</th>
						<th class="px-2 py-1 text-left font-bold">Payer</th>
						<th class="px-2 py-1 text-right font-bold">Amount</th>
						<th class="px-2 py-1 text-right font-bold">Date</th>
					</tr>
				</thead>
				<tbody>
					{#each (expenses ?? []) as exp, i}
						<tr class={i % 2 === 0 ? 'bg-win-panel' : 'bg-white'}>
							<td class="px-2 py-0.5">{exp.description}</td>
							<td class="px-2 py-0.5 text-win-dark">{exp.payer}</td>
							<td class="px-2 py-0.5 text-right font-mono">{formatCents(exp.total_cents)}</td>
							<td class="px-2 py-0.5 text-right text-win-dark">{formatDate(exp.created_at)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
			<a href="/expenses" class="block text-xs text-win-accent underline mt-2 font-system">
				View all expenses →
			</a>
		{/if}
	</Window>
	{/if}
{/if}
