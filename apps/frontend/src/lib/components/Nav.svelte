<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { logout } from '$lib/api/users';
	import { authStore } from '$lib/stores/auth';
	import Button from './Button.svelte';

	const links = [
		{ href: '/', label: 'HOME' },
		{ href: '/expenses', label: 'EXPENSES' },
		{ href: '/groups', label: 'GROUPS' },
		{ href: '/invitations', label: 'INVITATIONS' },
		{ href: '/settle', label: 'SETTLE UP' }
	];

	function handleLogout() {
		logout();
		goto('/login');
	}
</script>

<nav class="bg-win95 flex flex-wrap items-center gap-x-2 gap-y-1 px-2 sm:px-3 py-1" style="box-shadow: var(--bevel-out)">
	<span class="font-heading font-bold text-win-navy text-sm mr-3 shrink-0">Ardoise</span>

	{#each links as link}
		<a
			href={link.href}
			class="text-xs font-system font-bold uppercase px-2 py-0.5 shrink-0
			       {$page.url.pathname === link.href
				? 'underline text-win-navy'
				: 'text-black hover:underline'}"
		>
			{link.label}
		</a>
	{/each}

	<div class="ml-auto flex items-center gap-3 shrink-0">
		<a
			href="/profile"
			class="text-xs font-mono text-win-dark hidden sm:block hover:underline"
		>{$authStore.userID ?? ''}</a>
		<Button variant="danger" onclick={handleLogout}>LOGOUT</Button>
	</div>
</nav>
