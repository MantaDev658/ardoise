<script lang="ts">
	import { APIError } from '$lib/api/client';
	import { changePassword, updateUser } from '$lib/api/users';
	import Button from '$lib/components/Button.svelte';
	import HRule from '$lib/components/HRule.svelte';
	import Input from '$lib/components/Input.svelte';
	import Window from '$lib/components/Window.svelte';
	import { authStore } from '$lib/stores/auth';
	import { toastStore } from '$lib/stores/toast';

	// ── Display name ─────────────────────────────────────────────────
	let displayName = $state('');
	let savingName = $state(false);

	async function handleSaveName() {
		if (!displayName.trim() || savingName) return;
		savingName = true;
		try {
			await updateUser($authStore.userID!, displayName.trim());
			toastStore.success('Display name updated.');
			displayName = '';
		} catch (err) {
			toastStore.error(err instanceof APIError ? err.message : 'Failed to update name.');
		} finally {
			savingName = false;
		}
	}

	// ── Password change ───────────────────────────────────────────────
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let savingPassword = $state(false);

	async function handleChangePassword() {
		if (savingPassword) return;
		if (newPassword !== confirmPassword) {
			toastStore.error('New passwords do not match.');
			return;
		}
		savingPassword = true;
		try {
			await changePassword($authStore.userID!, currentPassword, newPassword);
			toastStore.success('Password changed.');
			// Clear all fields immediately — never leave passwords in state
			currentPassword = '';
			newPassword = '';
			confirmPassword = '';
		} catch (err) {
			toastStore.error(err instanceof APIError ? err.message : 'Failed to change password.');
		} finally {
			savingPassword = false;
		}
	}
</script>

<svelte:head>
	<title>Profile — Open Split</title>
</svelte:head>

<div class="max-w-sm flex flex-col gap-4">
	<Window title="PROFILE">
		<p class="font-system text-xs text-win-dark mb-3">
			Logged in as <span class="font-mono font-bold">{$authStore.userID}</span>
		</p>

		<!-- Display name -->
		<div class="flex flex-col gap-1 font-system">
			<label class="text-xs font-bold" for="profile-name">New display name</label>
			<div class="flex gap-2">
				<Input
					id="profile-name"
					bind:value={displayName}
					placeholder="Enter new name…"
					class="flex-1"
				/>
				<Button
					variant="success"
					onclick={handleSaveName}
					disabled={!displayName.trim() || savingName}
				>
					{savingName ? '…' : 'SAVE'}
				</Button>
			</div>
		</div>

		<HRule class="my-3" />

		<!-- Password change -->
		<div class="flex flex-col gap-2 font-system">
			<p class="text-xs font-bold uppercase">Change Password</p>

			<div class="flex flex-col gap-1">
				<label class="text-xs font-bold" for="profile-current-pw">Current password</label>
				<Input
					id="profile-current-pw"
					type="password"
					bind:value={currentPassword}
					placeholder="Current password"
					autocomplete="current-password"
				/>
			</div>

			<div class="flex flex-col gap-1">
				<label class="text-xs font-bold" for="profile-new-pw">New password</label>
				<Input
					id="profile-new-pw"
					type="password"
					bind:value={newPassword}
					placeholder="New password (min 8 chars)"
					autocomplete="new-password"
				/>
			</div>

			<div class="flex flex-col gap-1">
				<label class="text-xs font-bold" for="profile-confirm-pw">Confirm new password</label>
				<Input
					id="profile-confirm-pw"
					type="password"
					bind:value={confirmPassword}
					placeholder="Confirm new password"
					autocomplete="new-password"
				/>
			</div>

			<Button
				variant="success"
				onclick={handleChangePassword}
				disabled={!currentPassword || !newPassword || !confirmPassword || savingPassword}
				class="mt-1"
			>
				{savingPassword ? 'SAVING…' : 'CHANGE PASSWORD'}
			</Button>
		</div>
	</Window>
</div>
