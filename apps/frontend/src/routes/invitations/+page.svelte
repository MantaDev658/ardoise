<script lang="ts">
	import { onMount } from 'svelte';
	import { APIError } from '$lib/api/client';
	import { acceptInvitation, declineInvitation, listMyInvitations } from '$lib/api/invitations';
	import type { Invitation } from '$lib/api/types';
	import Button from '$lib/components/Button.svelte';
	import HRule from '$lib/components/HRule.svelte';
	import Window from '$lib/components/Window.svelte';
	import { toastStore } from '$lib/stores/toast';
	import { formatDate } from '$lib/utils';

	let invitations = $state<Invitation[]>([]);
	let loading = $state(true);
	let acting = $state<Record<string, boolean>>({});

	onMount(load);

	async function load() {
		loading = true;
		try {
			invitations = await listMyInvitations();
		} catch {
			toastStore.error('Failed to load invitations.');
		} finally {
			loading = false;
		}
	}

	async function handleAccept(inv: Invitation) {
		if (acting[inv.ID]) return;
		acting = { ...acting, [inv.ID]: true };
		try {
			await acceptInvitation(inv.ID);
			toastStore.success(`Joined "${inv.GroupName}".`);
			invitations = invitations.filter((i) => i.ID !== inv.ID);
		} catch (err) {
			toastStore.error(err instanceof APIError ? err.message : 'Failed to accept invitation.');
		} finally {
			acting = { ...acting, [inv.ID]: false };
		}
	}

	async function handleDecline(inv: Invitation) {
		if (acting[inv.ID]) return;
		acting = { ...acting, [inv.ID]: true };
		try {
			await declineInvitation(inv.ID);
			toastStore.success('Invitation declined.');
			invitations = invitations.filter((i) => i.ID !== inv.ID);
		} catch (err) {
			toastStore.error(err instanceof APIError ? err.message : 'Failed to decline invitation.');
		} finally {
			acting = { ...acting, [inv.ID]: false };
		}
	}
</script>

<svelte:head>
	<title>Invitations — Ardoise</title>
</svelte:head>

<Window title="INVITATIONS">
	{#if loading}
		<p class="font-system text-sm animate-pulse">Loading…</p>
	{:else if !invitations.length}
		<p class="font-system text-sm text-win-dark">No pending invitations.</p>
	{:else}
		<div class="flex flex-col gap-2 font-system text-sm">
			{#each invitations as inv, i}
				<div class="px-2 py-2 {i % 2 === 0 ? 'bg-win-panel' : 'bg-white'}">
					<div class="flex items-center justify-between gap-4">
						<div class="flex flex-col gap-0.5">
							<span class="font-bold">{inv.GroupName}</span>
							<span class="text-win-dark text-xs">
								Invited by {inv.InviterID} · {formatDate(inv.CreatedAt)}
							</span>
						</div>
						<div class="flex gap-2 shrink-0">
							<Button
								variant="success"
								onclick={() => handleAccept(inv)}
								disabled={acting[inv.ID]}
							>
								{acting[inv.ID] ? '…' : 'ACCEPT'}
							</Button>
							<Button
								variant="danger"
								onclick={() => handleDecline(inv)}
								disabled={acting[inv.ID]}
							>
								{acting[inv.ID] ? '…' : 'DECLINE'}
							</Button>
						</div>
					</div>
				</div>
				{#if i < invitations.length - 1}
					<HRule />
				{/if}
			{/each}
		</div>
	{/if}
</Window>
