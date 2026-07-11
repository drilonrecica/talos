<script lang="ts">
  import type { SessionInfo } from './auth';
  import { logout } from './auth';
  let { session, onlogout }: { session: SessionInfo; onlogout: () => void } =
    $props();
  let busy = $state(false);
  let error = $state('');

  async function end(all: boolean) {
    busy = true;
    error = '';
    try {
      await logout(all);
      onlogout();
    } catch (reason) {
      error = reason instanceof Error ? reason.message : 'Logout failed.';
      busy = false;
    }
  }
</script>

<details class="session-controls">
  <summary>{session.user.username}</summary>
  <p>Session expires {new Date(session.expiresAt).toLocaleString()}.</p>
  {#if error}<p role="alert">{error}</p>{/if}
  <button type="button" disabled={busy} onclick={() => end(false)}
    >Sign out</button
  >
  <button type="button" disabled={busy} onclick={() => end(true)}
    >Sign out everywhere</button
  >
</details>
