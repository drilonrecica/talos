<script lang="ts">
  import { tick } from 'svelte';
  import { claimSetup, verifySetupToken } from './onboarding';
  let { onclaimed }: { onclaimed: () => void } = $props();
  let token = $state('');
  let verified = $state(false);
  let username = $state('admin');
  let password = $state('');
  let confirmation = $state('');
  let busy = $state(false);
  let error = $state('');
  let alert = $state<HTMLElement>();

  async function showError(reason: unknown) {
    error = reason instanceof Error ? reason.message : 'Setup failed.';
    await tick();
    alert?.focus();
  }
  async function verify() {
    busy = true;
    error = '';
    try {
      await verifySetupToken(token);
      verified = true;
    } catch (reason) {
      await showError(reason);
    } finally {
      busy = false;
    }
  }
  async function claim() {
    if (password !== confirmation) {
      await showError(new Error('Passwords do not match.'));
      return;
    }
    busy = true;
    error = '';
    try {
      await claimSetup(token, username, password);
      onclaimed();
    } catch (reason) {
      await showError(reason);
    } finally {
      busy = false;
    }
  }
</script>

<section class="auth-card" aria-labelledby="setup-title">
  <h1 id="setup-title">Set up Binnacle</h1>
  <p>Enter the one-time token configured for this installation.</p>
  {#if error}<p bind:this={alert} tabindex="-1" role="alert">{error}</p>{/if}
  {#if !verified}
    <form
      onsubmit={(event) => {
        event.preventDefault();
        void verify();
      }}
    >
      <label for="setup-token">Setup token</label>
      <input
        id="setup-token"
        type="password"
        autocomplete="one-time-code"
        required
        bind:value={token}
      />
      <button disabled={busy}>Verify token</button>
    </form>
  {:else}
    <form
      onsubmit={(event) => {
        event.preventDefault();
        void claim();
      }}
    >
      <label for="setup-username">Administrator username</label>
      <input
        id="setup-username"
        autocomplete="username"
        required
        bind:value={username}
      />
      <label for="setup-password">Password</label>
      <input
        id="setup-password"
        type="password"
        minlength="12"
        maxlength="128"
        autocomplete="new-password"
        required
        bind:value={password}
      />
      <label for="setup-confirmation">Confirm password</label>
      <input
        id="setup-confirmation"
        type="password"
        autocomplete="new-password"
        required
        bind:value={confirmation}
      />
      <button disabled={busy}>Create administrator</button>
    </form>
  {/if}
</section>
