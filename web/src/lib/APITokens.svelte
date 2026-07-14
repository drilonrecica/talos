<script lang="ts">
  import { onMount } from 'svelte';
  import { authenticatedMutation } from './auth';

  type Scope =
    | 'server:read'
    | 'resources:read'
    | 'metrics:read'
    | 'events:read'
    | 'incidents:read';
  interface Token {
    id: string;
    name: string;
    prefix: string;
    scopes: Scope[];
    createdAt: string;
    expiresAt?: string;
    lastUsedAt?: string;
    revokedAt?: string;
  }
  const labels: Record<Scope, string> = {
    'server:read': 'Server',
    'resources:read': 'Resources',
    'metrics:read': 'Metrics',
    'events:read': 'Events',
    'incidents:read': 'Incidents',
  };
  let tokens = $state<Token[]>([]);
  let scopes = $state<Scope[]>([]);
  let selected = $state<Scope[]>(['server:read']);
  let name = $state('');
  let expiry = $state('none');
  let plaintext = $state('');
  let busy = $state(false);
  let error = $state('');

  onMount(() => void load());
  async function load() {
    const response = await fetch('/api/v1/api-tokens', {
      credentials: 'same-origin',
    });
    if (!response.ok) {
      error = 'API tokens could not be loaded.';
      return;
    }
    const body = (await response.json()) as {
      tokens: Token[];
      scopes: Scope[];
    };
    tokens = body.tokens;
    scopes = body.scopes;
  }
  function toggle(scope: Scope, checked: boolean) {
    selected = checked
      ? [...selected, scope]
      : selected.filter((value) => value !== scope);
  }
  async function create() {
    busy = true;
    error = '';
    plaintext = '';
    try {
      const expiresAt =
        expiry === 'none'
          ? undefined
          : new Date(Date.now() + Number(expiry) * 86400000).toISOString();
      const result = await authenticatedMutation<{
        token: Token;
        plaintext: string;
      }>('/api/v1/api-tokens', 'POST', { name, scopes: selected, expiresAt });
      if (!result) throw new Error('API token could not be created.');
      tokens = [result.token, ...tokens];
      plaintext = result.plaintext;
      name = '';
    } catch (reason) {
      error = reason instanceof Error ? reason.message : String(reason);
    } finally {
      busy = false;
    }
  }
  async function revoke(token: Token) {
    if (!confirm(`Revoke API token “${token.name}”?`)) return;
    busy = true;
    try {
      await authenticatedMutation(`/api/v1/api-tokens/${token.id}`, 'DELETE');
      tokens = tokens.map((value) =>
        value.id === token.id
          ? { ...value, revokedAt: new Date().toISOString() }
          : value,
      );
    } catch (reason) {
      error = reason instanceof Error ? reason.message : String(reason);
    } finally {
      busy = false;
    }
  }
</script>

<h3>Personal API tokens</h3>
<p>
  Read-only tokens are shown once. Diagnostics and settings remain session-only.
</p>
{#if error}<p role="alert">{error}</p>{/if}
{#if plaintext}
  <div role="status">
    <strong>Copy this token now. It will not be shown again.</strong>
    <code>{plaintext}</code>
    <button
      type="button"
      onclick={() => navigator.clipboard.writeText(plaintext)}>Copy</button
    >
  </div>
{/if}
<form
  onsubmit={(event) => {
    event.preventDefault();
    void create();
  }}
>
  <label for="api-token-name">Name</label>
  <input id="api-token-name" required maxlength="64" bind:value={name} />
  <fieldset>
    <legend>Scopes</legend>
    {#each scopes as scope}
      <label>
        <input
          type="checkbox"
          checked={selected.includes(scope)}
          onchange={(event) => toggle(scope, event.currentTarget.checked)}
        />
        {labels[scope]}
      </label>
    {/each}
  </fieldset>
  <label for="api-token-expiry">Expiry</label>
  <select id="api-token-expiry" bind:value={expiry}>
    <option value="none">No expiry</option><option value="1">1 day</option
    ><option value="30">30 days</option><option value="365">1 year</option>
  </select>
  <button type="submit" disabled={busy || !name.trim() || !selected.length}
    >Create token</button
  >
</form>
{#if tokens.length}
  <table>
    <thead
      ><tr
        ><th>Name</th><th>Prefix</th><th>Scopes</th><th>Last used</th><th
          >Status</th
        ><th></th></tr
      ></thead
    >
    <tbody>
      {#each tokens as token (token.id)}
        <tr>
          <td>{token.name}</td><td><code>{token.prefix}…</code></td><td
            >{token.scopes.join(', ')}</td
          ><td
            >{token.lastUsedAt
              ? new Date(token.lastUsedAt).toLocaleString()
              : 'Never'}</td
          ><td>{token.revokedAt ? 'Revoked' : 'Active'}</td><td
            ><button
              type="button"
              disabled={busy || !!token.revokedAt}
              onclick={() => revoke(token)}>Revoke</button
            ></td
          >
        </tr>
      {/each}
    </tbody>
  </table>
{/if}
