<script lang="ts">
  import { onMount } from 'svelte';
  import type { LiveSnapshot } from './live.svelte';
  import {
    loadServerPreferences,
    preferences,
    saveServerPreferences,
    type ChartRange,
    type Density,
    type LandingPage,
    type Theme,
    type UserPreferences,
  } from './preferences';

  let value = $state<UserPreferences>(preferences());
  let resources = $state<LiveSnapshot['resources']>([]);
  let selected = $state('');
  let busy = $state(false);
  let error = $state('');

  onMount(() => {
    void loadServerPreferences()
      .then((saved) => (value = saved))
      .catch((reason) => (error = String(reason)));
    void fetch('/api/v1/resources', { credentials: 'same-origin' })
      .then(async (response) => {
        if (response.ok)
          resources = (await response.json()) as LiveSnapshot['resources'];
      })
      .catch(() => undefined);
  });

  async function save(next: UserPreferences) {
    busy = true;
    error = '';
    try {
      value = await saveServerPreferences(next);
    } catch (reason) {
      error = reason instanceof Error ? reason.message : String(reason);
    } finally {
      busy = false;
    }
  }
  function update(changes: Partial<UserPreferences>) {
    void save({ ...value, ...changes });
  }
  function pin() {
    if (!selected || value.pinnedResources.includes(selected)) return;
    if (value.pinnedResources.length >= 12) {
      error = 'At most 12 resources can be pinned.';
      return;
    }
    update({ pinnedResources: [...value.pinnedResources, selected] });
    selected = '';
  }
  function move(index: number, offset: number) {
    const target = index + offset;
    if (target < 0 || target >= value.pinnedResources.length) return;
    const pins = [...value.pinnedResources];
    [pins[index], pins[target]] = [pins[target], pins[index]];
    update({ pinnedResources: pins });
  }
  function resourceName(id: string) {
    return resources.find((resource) => resource.id === id)?.name ?? id;
  }
</script>

{#if error}<p role="alert">{error}</p>{/if}
<label for="settings-theme">Theme</label>
<select
  id="settings-theme"
  value={value.theme}
  disabled={busy}
  onchange={(event) => update({ theme: event.currentTarget.value as Theme })}
>
  <option value="system">System</option><option value="dark">Dark</option
  ><option value="light">Light</option>
</select>
<label for="settings-density">Density</label>
<select
  id="settings-density"
  value={value.density}
  disabled={busy}
  onchange={(event) =>
    update({ density: event.currentTarget.value as Density })}
>
  <option value="comfortable">Comfortable</option><option value="compact"
    >Compact</option
  >
</select>
<label for="settings-landing">Default landing page</label>
<select
  id="settings-landing"
  value={value.landingPage}
  disabled={busy}
  onchange={(event) =>
    update({ landingPage: event.currentTarget.value as LandingPage })}
>
  <option value="watch">Watch</option><option value="resources"
    >Resources</option
  ><option value="server">Server</option><option value="events">Events</option
  ><option value="alerts">Alerts</option>
</select>
<label for="settings-chart-range">Default chart range</label>
<select
  id="settings-chart-range"
  value={value.chartRange}
  disabled={busy}
  onchange={(event) =>
    update({ chartRange: event.currentTarget.value as ChartRange })}
>
  {#each ['1h', '6h', '24h', '7d', '30d'] as range}<option value={range}
      >{range}</option
    >{/each}
</select>

<h3>Pinned resources</h3>
<p>Pinned active resources appear first on Watch. Missing pins are ignored.</p>
<div class="inline-controls">
  <label for="pin-resource">Resource</label>
  <select id="pin-resource" bind:value={selected} disabled={busy}>
    <option value="">Select a resource</option>
    {#each resources.filter((resource) => !value.pinnedResources.includes(resource.id)) as resource (resource.id)}
      <option value={resource.id}>{resource.name}</option>
    {/each}
  </select>
  <button type="button" disabled={!selected || busy} onclick={pin}>Pin</button>
</div>
{#if value.pinnedResources.length}
  <ol>
    {#each value.pinnedResources as id, index (id)}
      <li>
        {resourceName(id)}
        <button
          type="button"
          disabled={busy || index === 0}
          aria-label={`Move ${resourceName(id)} up`}
          onclick={() => move(index, -1)}>↑</button
        >
        <button
          type="button"
          disabled={busy || index === value.pinnedResources.length - 1}
          aria-label={`Move ${resourceName(id)} down`}
          onclick={() => move(index, 1)}>↓</button
        >
        <button
          type="button"
          disabled={busy}
          onclick={() =>
            update({
              pinnedResources: value.pinnedResources.filter(
                (candidate) => candidate !== id,
              ),
            })}>Remove</button
        >
      </li>
    {/each}
  </ol>
{/if}
