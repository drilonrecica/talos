<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { AuthError, login, safeRedirect } from './auth';
  let { onauthenticated }: { onauthenticated: (path: string) => void } =
    $props();
  let username = $state('');
  let password = $state('');
  let error = $state('');
  let busy = $state(false);
  let errorElement = $state<HTMLElement>();
  let usernameElement = $state<HTMLInputElement>();
  onMount(() => usernameElement?.focus());

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    busy = true;
    error = '';
    try {
      await login(username, password);
      onauthenticated(
        safeRedirect(new URLSearchParams(location.search).get('next')),
      );
    } catch (reason) {
      const authError = reason as AuthError;
      error = authError.retryAfterSeconds
        ? `${authError.message} Retry in ${authError.retryAfterSeconds} seconds.`
        : authError.message;
      await tick();
      errorElement?.focus();
    } finally {
      busy = false;
    }
  }
</script>

<section class="auth-card" aria-labelledby="login-title">
  <h1 id="login-title">Sign in to TALOS</h1>
  <p>Use the local administrator account for this server.</p>
  {#if error}<p bind:this={errorElement} tabindex="-1" role="alert">
      {error}
    </p>{/if}
  <form onsubmit={submit} aria-busy={busy}>
    <label for="username">Username</label>
    <input
      bind:this={usernameElement}
      id="username"
      name="username"
      autocomplete="username"
      required
      bind:value={username}
    />
    <label for="password">Password</label>
    <input
      id="password"
      name="password"
      type="password"
      autocomplete="current-password"
      required
      bind:value={password}
    />
    <button type="submit" disabled={busy}
      >{busy ? 'Signing in…' : 'Sign in'}</button
    >
  </form>
</section>
