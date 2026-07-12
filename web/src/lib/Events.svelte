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
  import ConsoleSection from './ui/ConsoleSection.svelte';
  import ConsoleState from './ui/ConsoleState.svelte';

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

<section class="console-page" aria-labelledby="events-title">
  <ConsoleSection
    code="LOG"
    title="Event logbook"
    id="events-title"
    detail={`${live.events.length} live / ${events.length} historical`}
  />
  <div class="control-rail">
    <span>RANGE</span>
    <label>
      Range
      <select bind:value={range} onchange={() => load()}>
        {#each ranges as r (r)}<option value={r}>Last {r}</option>{/each}
      </select>
    </label>
    <button onclick={() => load()} disabled={loading}>Refresh</button>
  </div>

  {#if live.events.length}<table class="console-table event-log live-log">
      <caption>Live events</caption><thead
        ><tr
          ><th>Time</th><th>Severity</th><th>Type</th><th>Summary</th><th
            >Source</th
          ></tr
        ></thead
      ><tbody>
        {#each live.events.slice().reverse() as event (event.id)}<tr
            ><td><span class="live-mark">LIVE</span></td><td
              ><ConsoleState state="healthy" label="live" /></td
            ><td>{event.type}</td><td>{event.message}</td><td
              >{event.resourceId ?? 'stream'}</td
            ></tr
          >{/each}
      </tbody>
    </table>{/if}

  {#if loading}<Loading />{:else if error}<Alert level="error">{error}</Alert
    >{:else if events.length === 0}<EmptyState title="No events"
      ><p>No events for the selected range.</p></EmptyState
    >{:else}
    <div class="table-scroll">
      <table class="console-table event-log">
        <caption>Historical events</caption>
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
                ><ConsoleState
                  state={severityState(event.severity)}
                  label={event.severity}
                /></td
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
    </div>
  {/if}
</section>
