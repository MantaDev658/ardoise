<script lang="ts">
	let {
		label,
		value,
		class: className = ''
	}: {
		label: string;
		value: number;
		class?: string;
	} = $props();

	// Zero-pad to 8 digits, display absolute value — caller controls sign semantics
	const display = $derived(String(Math.abs(value)).padStart(8, '0'));
</script>

<div class="{className} bg-black p-3" style="box-shadow: var(--bevel-in)">
	<div class="font-mono text-win-green text-lg sm:text-2xl tracking-widest select-none">{display}</div>
	<div class="font-system text-xs text-win-dark uppercase mt-1">{label}</div>
</div>
