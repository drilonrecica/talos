<script lang="ts">
  import { onMount } from 'svelte';
  import { SvelteURLSearchParams } from 'svelte/reactivity';
  import ConsoleSection from './ui/ConsoleSection.svelte';
  import Alert from './ui/Alert.svelte';

  type Component = { id: string; name: string };
  type Resource = { id: string; name: string; components?: Component[] };
  type Entry = {
    timestamp: string;
    component: string;
    stream: string;
    severity: string;
    message: string;
  };
  let resources = $state<Resource[]>([]);
  let resource = $state('');
  let component = $state('');
  let range = $state('5m');
  let search = $state('');
  let follow = $state(false);
  let highlighting = $state(true);
  let entries = $state<Entry[]>([]);
  let truncated = $state(false);
  let loading = $state(false);
  let error = $state('');
  let source: EventSource | null = null;
  let selected = $derived(resources.find((value) => value.id === resource));

  onMount(async () => {
    const response = await fetch('/api/v1/resources');
    if (!response.ok) return;
    resources = await response.json();
    const query = new SvelteURLSearchParams(location.search);
    resource = query.get('resource') || resources[0]?.id || '';
    component = query.get('container') || '';
    const at = query.get('at');
    if (at) range = '5m';
  });

  function url(streaming = false) {
    const query = new SvelteURLSearchParams({ range, limit: '500' });
    if (component) query.set('container', component);
    else query.set('resource', resource);
    if (search) query.set('search', search);
    if (streaming) query.set('follow', 'true');
    return `/api/v1/logs?${query}`;
  }

  async function load() {
    source?.close();
    entries = [];
    truncated = false;
    error = '';
    if (!resource && !component) return;
    if (follow) {
      source = new EventSource(url(true));
      source.addEventListener('log', (event) => {
        entries = [
          ...entries.slice(-499),
          JSON.parse((event as MessageEvent).data),
        ];
      });
      source.addEventListener('end', () => source?.close());
      source.onerror = () => {
        error = 'Live log stream ended.';
        source?.close();
      };
      return;
    }
    loading = true;
    try {
      const response = await fetch(url());
      if (!response.ok) throw new Error('Logs are unavailable.');
      const body = await response.json();
      entries = body.entries;
      truncated = body.truncated;
    } catch (reason) {
      error =
        reason instanceof Error ? reason.message : 'Logs are unavailable.';
    } finally {
      loading = false;
    }
  }
</script>

<section class="console-page" aria-labelledby="logs-title">
  <ConsoleSection
    code="DIAG"
    title="Container logs"
    id="logs-title"
    detail="ephemeral / best-effort redaction"
  />
  <form
    class="control-rail"
    onsubmit={(event) => {
      event.preventDefault();
      void load();
    }}
  >
    <label
      >Resource<select bind:value={resource} onchange={() => (component = '')}>
        {#each resources as value (value.id)}<option value={value.id}
            >{value.name}</option
          >{/each}
      </select></label
    >
    <label
      >Component<select bind:value={component}
        ><option value="">All components</option>
        {#each selected?.components ?? [] as value (value.id)}<option
            value={value.id}>{value.name}</option
          >{/each}
      </select></label
    >
    <label
      >Range<select bind:value={range}
        ><option value="5m">5 minutes</option><option value="30m"
          >30 minutes</option
        ><option value="1h">1 hour</option></select
      ></label
    >
    <label>Literal search<input bind:value={search} maxlength="256" /></label>
    <label><input type="checkbox" bind:checked={follow} /> Follow</label>
    <label
      ><input type="checkbox" bind:checked={highlighting} /> Severity color</label
    >
    <button type="submit" disabled={loading}>{follow ? 'Start' : 'Load'}</button
    >
  </form>
  {#if error}<Alert level="warning">{error}</Alert>{/if}
  {#if truncated}<Alert level="warning"
      >Output reached the configured line or byte limit.</Alert
    >{/if}
  <div class="table-scroll" aria-live="polite">
    <table class="console-table event-log">
      <caption>{entries.length} redacted log lines</caption>
      <thead
        ><tr><th>Time</th><th>Stream</th><th>Component</th><th>Message</th></tr
        ></thead
      >
      <tbody
        >{#each entries as entry, index (`${entry.timestamp}-${index}`)}<tr
            data-severity={highlighting ? entry.severity : 'unknown'}
          >
            <td
              >{entry.timestamp
                ? new Date(entry.timestamp).toLocaleTimeString()
                : '—'}</td
            ><td>{entry.stream}</td><td>{entry.component.slice(0, 12)}</td><td
              class="technical-value">{entry.message}</td
            >
          </tr>{/each}</tbody
      >
    </table>
  </div>
  <p class="meta">
    Logs are not persisted. Redaction is best-effort; avoid logging secrets.
  </p>
</section>
