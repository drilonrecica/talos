<script lang="ts">
  import { onMount } from 'svelte';
  import type { LiveStore } from './live.svelte';
  import {
    fetchEventHistory,
    eventRangeFor,
    type EventRangeKey,
  } from './events';
  import Loading from './ui/Loading.svelte';
  import EmptyState from './ui/EmptyState.svelte';
  import Alert from './ui/Alert.svelte';
  import Badge from './ui/Badge.svelte';

  let { live }: { live: LiveStore } = $props();

  const ranges: EventRangeKey[] = ['1h', '6h', '24h', '7d'];
  let range = $state<EventRangeKey>('24h');
  let events = $state<Awaited<ReturnType<typeof fetchEventHistory>>>([]);
  let loading = $state(false);
  let error = $state<string | null>(null);

  async function load() {
    loading = true;
    error = null;
    try {
      const { from, to } = eventRangeFor(range);
      events = await fetchEventHistory('host', undefined, from, to);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load events.';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void load();
  });

  function formatTs(iso: string) {
    return new Date(iso).toLocaleString();
  }

  function severityState(severity: string): string {
    switch (severity) {
      case 'critical':
        return 'error';
      case 'warning':
        return 'warning';
      default:
        return 'info';
    }
  }
</script>

<section>
  <h2>Event history</h2>
  <div class="controls">
    <label>
      Range
      <select bind:value={range} onchange={() => load()}>
        {#each ranges as r (r)}<option value={r}>Last {r}</option>{/each}
      </select>
    </label>
    <button onclick={() => load()} disabled={loading}>Refresh</button>
  </div>

  {#if live.events.length}
    <h3>Live</h3>
    <ul class="live-events">
      {#each live.events.slice().reverse() as event (event.id)}
        <li><Badge state="info">{event.type}</Badge> {event.message}</li>
      {/each}
    </ul>
  {/if}

  {#if loading}<Loading />{:else if error}<Alert level="error">{error}</Alert
    >{:else if events.length === 0}<EmptyState title="No events"
      ><p>No events for the selected range.</p></EmptyState
    >{:else}
    <table class="events-table">
      <thead>
        <tr
          ><th>Time</th><th>Severity</th><th>Type</th><th>Summary</th><th
            >Source</th
          ></tr
        >
      </thead>
      <tbody>
        {#each events as event (event.id)}
          <tr>
            <td><time>{formatTs(event.ts)}</time></td>
            <td
              ><Badge state={severityState(event.severity)}
                >{event.severity}</Badge
              ></td
            >
            <td>{event.type}</td>
            <td>
              {event.summary}
              {#if event.resourceId}<span class="meta"
                  >resource {event.resourceId}</span
                >{/if}
              {#if event.containerInstanceId}<span class="meta"
                  >container {event.containerInstanceId}</span
                >{/if}
              {#if event.correlationKey}<span class="meta"
                  >corr {event.correlationKey}</span
                >{/if}
            </td>
            <td>{event.source}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</section>

<style>
  .controls {
    display: flex;
    gap: 1rem;
    align-items: center;
    margin-bottom: 1rem;
  }
  .live-events {
    list-style: none;
    padding: 0;
    margin: 0 0 1.5rem;
  }
  .live-events li {
    margin-bottom: 0.25rem;
  }
  .events-table {
    width: 100%;
    border-collapse: collapse;
  }
  .events-table th,
  .events-table td {
    text-align: left;
    padding: 0.5rem;
    border-bottom: 1px solid var(--color-border, #ddd);
  }
  .meta {
    display: inline-block;
    margin-left: 0.5rem;
    font-size: 0.8rem;
    color: var(--color-text-muted, #666);
  }
  time {
    white-space: nowrap;
    color: var(--color-text-muted, #666);
  }
</style>
