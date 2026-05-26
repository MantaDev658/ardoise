<script lang="ts">
	import '../app.css';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import Marquee from '$lib/components/Marquee.svelte';
	import Nav from '$lib/components/Nav.svelte';
	import Toast from '$lib/components/Toast.svelte';
	import { authStore } from '$lib/stores/auth';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	const PUBLIC = new Set(['/login', '/register']);

	const authenticated = $derived(!!$authStore.token && !PUBLIC.has($page.url.pathname));

	$effect(() => {
		if (!$authStore.token && !PUBLIC.has($page.url.pathname)) {
			goto('/login');
		}
	});
</script>

{#if authenticated}
	<Marquee
		text="★ WELCOME TO Ardoise ★ YOUR BALANCES AWAIT ★ SPLIT SMART, SETTLE FAST ★ EST. 2025 ★"
	/>
	<Nav />
{/if}
<main class={authenticated ? 'bg-90s-tile min-h-screen p-4' : ''}>
	{@render children()}
</main>

<Toast />
