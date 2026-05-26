<script lang="ts">
	import { goto } from '$app/navigation';
	import { APIError } from '$lib/api/client';
	import { login, register } from '$lib/api/users';
	import Button from '$lib/components/Button.svelte';
	import Input from '$lib/components/Input.svelte';
	import Window from '$lib/components/Window.svelte';
	import { authStore } from '$lib/stores/auth';

	let id = $state('');
	let displayName = $state('');
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
			await register(id, displayName, password);
			// Auto-login after registration so the user lands on the dashboard
			await login(id, password);
			goto('/');
		} catch (err) {
			error =
				err instanceof APIError
					? err.message
					: 'Something went wrong. Please try again.';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Register — Ardoise</title>
</svelte:head>

<div class="min-h-screen bg-90s-tile flex items-center justify-center p-4">
	<div class="w-80">
		<Window title="Ardoise v1.0 — Register">
			<form class="flex flex-col gap-3 font-system" onsubmit={handleSubmit}>
				<div class="flex flex-col gap-1">
					<label class="text-sm font-bold" for="reg-id">Username</label>
					<Input
						id="reg-id"
						bind:value={id}
						placeholder="choose-a-username"
						autocomplete="username"
					/>
					<span class="text-xs text-win-dark">Used to log in. Cannot be changed.</span>
				</div>

				<div class="flex flex-col gap-1">
					<label class="text-sm font-bold" for="reg-display-name">Display Name</label>
					<Input
						id="reg-display-name"
						bind:value={displayName}
						placeholder="Your Name"
						autocomplete="name"
					/>
				</div>

				<div class="flex flex-col gap-1">
					<label class="text-sm font-bold" for="reg-password">Password</label>
					<Input
						id="reg-password"
						type="password"
						bind:value={password}
						autocomplete="new-password"
					/>
				</div>

				{#if error}
					<p class="text-xs text-win-red px-2 py-1" style="box-shadow: var(--bevel-in)">
						⚠ {error}
					</p>
				{/if}

				<div class="flex items-center justify-between pt-1">
					<Button type="submit" variant="primary" disabled={loading}>
						{loading ? 'REGISTERING…' : 'REGISTER'}
					</Button>
					<a href="/login" class="text-xs text-win-accent underline">
						Have an account? Login
					</a>
				</div>
			</form>
		</Window>
	</div>
</div>
