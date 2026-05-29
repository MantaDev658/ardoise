<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { APIError } from '$lib/api/client';
	import { listGroups } from '$lib/api/groups';
	import { createSettlement } from '$lib/api/settlements';
	import type { Group, User } from '$lib/api/types';
	import { listFriends } from '$lib/api/users';
	import Button from '$lib/components/Button.svelte';
	import HRule from '$lib/components/HRule.svelte';
	import Input from '$lib/components/Input.svelte';
	import Select from '$lib/components/Select.svelte';
	import Window from '$lib/components/Window.svelte';
	import { authStore } from '$lib/stores/auth';
	import { toastStore } from '$lib/stores/toast';

	let users = $state<User[]>([]);
	let groups = $state<Group[]>([]);
	let loading = $state(true);

	let receiverID = $state('');
	let amountDollars = $state('');
	let groupID = $state('');
	let submitting = $state(false);

	const receiverOptions = $derived(
		users
			.filter((u) => u.ID !== $authStore.userID)
			.map((u) => ({ value: u.ID, label: u.DisplayName }))
	);

	const groupOptions = $derived(
		groups.map((g) => ({ value: g.ID, label: g.Name }))
	);

	onMount(() => {
		const params = $page.url.searchParams;
		const toParam = params.get('to');
		const amountParam = params.get('amount');
		const groupParam = params.get('group');

		Promise.all([listFriends(), listGroups()])
			.then(([usrs, grps]) => {
				users = usrs ?? [];
				groups = grps ?? [];
				if (toParam) receiverID = toParam;
				if (amountParam) amountDollars = (parseInt(amountParam, 10) / 100).toFixed(2);
				if (groupParam) groupID = groupParam;
			})
			.catch(() => toastStore.error('Failed to load friends and groups.'))
			.finally(() => (loading = false));
	});

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (submitting) return;

		const amountCents = Math.round(parseFloat(amountDollars) * 100);
		if (!receiverID) { toastStore.error('Select a recipient.'); return; }
		if (isNaN(amountCents) || amountCents <= 0) { toastStore.error('Enter a valid amount.'); return; }

		submitting = true;
		try {
			await createSettlement({
				receiver_id: receiverID,
				amount_cents: amountCents,
				...(groupID ? { group_id: groupID } : {})
			});
			toastStore.success('Settlement recorded!');
			const dest = groupID ? `/groups/${groupID}` : '/';
			receiverID = '';
			amountDollars = '';
			groupID = '';
			goto(dest);
		} catch (err) {
			toastStore.error(err instanceof APIError ? err.message : 'Failed to record settlement.');
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>Settle Up — Ardoise</title>
</svelte:head>

<div class="max-w-sm">
	<Window title="SETTLE UP">
		{#if !loading && users.length === 0}
			<p class="font-system text-xs text-center py-2">
				You have no friends yet — join a group first.
			</p>
		{:else}
		<form class="flex flex-col gap-3 font-system" onsubmit={handleSubmit}>
			<div class="flex flex-col gap-1">
				<label class="text-xs font-bold" for="settle-receiver">Send payment to</label>
				<Select
					id="settle-receiver"
					bind:value={receiverID}
					placeholder={loading ? 'Loading…' : 'Select recipient…'}
					options={receiverOptions}
					disabled={loading}
				/>
			</div>

			<div class="flex flex-col gap-1">
				<label class="text-xs font-bold" for="settle-amount">Amount ($)</label>
				<Input
					id="settle-amount"
					type="number"
					min="0.01"
					step="0.01"
					bind:value={amountDollars}
					placeholder="0.00"
					disabled={loading}
				/>
			</div>

			<div class="flex flex-col gap-1">
				<label class="text-xs font-bold" for="settle-group">Group (optional)</label>
				<Select
					id="settle-group"
					bind:value={groupID}
					placeholder="No group"
					options={groupOptions}
					disabled={loading}
				/>
			</div>

			<HRule />

			<Button type="submit" variant="success" disabled={submitting || loading} class="w-full py-2 text-base">
				{submitting ? 'RECORDING…' : 'SETTLE UP'}
			</Button>
		</form>
		{/if}

		<!-- Warning stripe -->
		<div class="bg-construction h-5 mt-4" aria-hidden="true"></div>
		<p class="font-system text-xs font-bold text-center py-1 bg-win-yellow">
			⚠ THIS ACTION CANNOT BE UNDONE ⚠
		</p>
	</Window>
</div>
