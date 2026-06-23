<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		title,
		children,
		icon,
		class: className = ''
	}: {
		title: string;
		children: Snippet;
		icon?: Snippet;
		class?: string;
	} = $props();
</script>

<div class="bg-win95 {className}" style="box-shadow: var(--bevel-out-deep); padding: 2px">
	<div
		class="flex items-center gap-1.5 px-1.5 py-1 select-none"
		style="background: linear-gradient(to right, #000080, #1084d0)"
	>
		{#if icon}
			{@render icon()}
		{/if}
		<span class="text-white font-bold text-sm font-system flex-1 truncate">{title}</span>
	</div>

	<div class="bg-white m-0.5 p-2 sm:p-4" style="box-shadow: var(--bevel-in)">
		{@render children()}
	</div>
</div>
