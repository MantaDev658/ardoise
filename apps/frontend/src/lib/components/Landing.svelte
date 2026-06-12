<script lang="ts">
	import Button from './Button.svelte';
	import Marquee from './Marquee.svelte';
	import Window from './Window.svelte';
	import { goto } from '$app/navigation';

	const features = [
		{ title: 'GROUPS', body: 'Make a group for a trip, a flat, or a night out — then track who paid for what.' },
		{ title: 'SMART SPLITS', body: 'Split evenly, by exact amounts, by percentage, or by shares. The cents always add up.' },
		{ title: 'SETTLE UP FAST', body: 'We work out the fewest payments needed to clear every debt in the group.' },
		{ title: 'OPEN SOURCE', body: 'Use the hosted app, or run your own copy — the code is free and open.' }
	];
</script>

<div class="min-h-screen bg-90s-tile flex flex-col items-center p-4 gap-4">
	<div class="w-full max-w-3xl">
		<Marquee text="★ WELCOME TO ARDOISE ★ SPLIT SMART, SETTLE FAST ★ KEEP A RUNNING TAB ★ EST. 2025 ★" />
	</div>

	<!-- Hero -->
	<div class="w-full max-w-3xl">
		<Window title="Ardoise v1.0">
			<div class="font-system text-sm flex flex-col gap-4 items-center text-center py-2">
				<h1 class="font-heading text-win-navy text-3xl sm:text-4xl">ARDOISE</h1>
				<p class="font-bold text-base">Split shared expenses with friends and settle up the easy way.</p>
				<p class="text-win-dark max-w-md">
					Track what everyone paid, split costs fairly, and find the simplest way to settle up.
					Sign up to use the hosted app — or, since it's open source, run your own copy.
				</p>
				<p class="text-win-dark text-xs max-w-md italic">
					“Ardoise” is French slang for a running tab — the slate a café keeps of what you owe.
				</p>
				<div class="flex items-center gap-3 pt-1">
					<Button variant="primary" onclick={() => goto('/login')}>LOGIN</Button>
					<Button variant="default" onclick={() => goto('/register')}>REGISTER</Button>
				</div>
				<a
					href="https://github.com/MantaDev658/ardoise"
					class="text-xs text-win-accent underline"
				>
					View source on GitHub →
				</a>
			</div>
		</Window>
	</div>

	<!-- Features -->
	<div class="w-full max-w-3xl grid grid-cols-1 sm:grid-cols-2 gap-4">
		{#each features as f}
			<Window title={f.title}>
				<p class="font-system text-sm text-win-dark">{f.body}</p>
			</Window>
		{/each}
	</div>

	<p class="font-system text-xs text-white pb-2">Open source · AGPL-3.0</p>
</div>
