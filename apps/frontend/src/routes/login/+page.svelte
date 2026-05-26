<script lang="ts">
	import { goto } from '$app/navigation';
	import { APIError } from '$lib/api/client';
	import { login } from '$lib/api/users';
	import Button from '$lib/components/Button.svelte';
	import Input from '$lib/components/Input.svelte';
	import Window from '$lib/components/Window.svelte';
	import { authStore } from '$lib/stores/auth';

	let id = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	$effect(() => {
		if ($authStore.token) goto('/');
	});

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (loading) return;
		loading = true;
		error = '';

		try {
			await login(id, password);
			goto('/');
		} catch (err) {
			error =
				err instanceof APIError && err.status === 401
					? 'Invalid username or password.'
					: 'Something went wrong. Please try again.';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Login — Ardoise</title>
</svelte:head>

<div class="min-h-screen bg-90s-tile flex items-center justify-center p-4">
	<div class="w-80">
		<Window title="Ardoise v1.0 — Login">
			<form class="flex flex-col gap-3 font-system" onsubmit={handleSubmit}>
				<div class="flex flex-col gap-1">
					<label class="text-sm font-bold" for="login-id">Username</label>
					<Input id="login-id" bind:value={id} placeholder="your-username" autocomplete="username" />
				</div>

				<div class="flex flex-col gap-1">
					<label class="text-sm font-bold" for="login-password">Password</label>
					<Input
						id="login-password"
						type="password"
						bind:value={password}
						autocomplete="current-password"
					/>
				</div>

				{#if error}
					<p class="text-xs text-win-red px-2 py-1" style="box-shadow: var(--bevel-in)">
						⚠ {error}
					</p>
				{/if}

				<div class="flex items-center justify-between pt-1">
					<Button type="submit" variant="primary" disabled={loading}>
						{loading ? 'LOGGING IN…' : 'LOGIN'}
					</Button>
					<a href="/register" class="text-xs text-win-accent underline">
						No account? Register
					</a>
				</div>
			</form>
		</Window>
	</div>
</div>
