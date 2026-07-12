<script lang="ts">
  import { onMount } from 'svelte';
  import type { LiveStore } from './live.svelte';
  import { formatBytes, formatNumber } from './i18n';
  import ConsoleSection from './ui/ConsoleSection.svelte';
  import ConsoleState from './ui/ConsoleState.svelte';
  import HistoryCharts from './HistoryCharts.svelte';
  import HistoryDeletion from './HistoryDeletion.svelte';
  let { live, id }: { live: LiveStore; id: string } = $props();
  let current = $derived(
    live.snapshot?.resources.find((value) => value.id === id),
  );
  let archived = $state<{
    id: string;
    name: string;
    status: string;
    category: string;
    project?: string;
    environment?: string;
    archivedAt?: string;
  } | null>(null);
  let error = $state('');
  onMount(() => {
    if (current) return;
    void fetch(`/api/v1/resources/${encodeURIComponent(id)}`, {
      credentials: 'same-origin',
    })
      .then((response) => {
        if (!response.ok) throw new Error('Resource unavailable.');
        return response.json();
      })
      .then((value) => (archived = value))
      .catch((reason) => (error = String(reason)));
  });
</script>

{#if current}
  <section
    class="console-page resource-detail"
    aria-labelledby="resource-title"
  >
    <header class="identity-strip">
      <div>
        <span>RESOURCE</span>
        <h1 id="resource-title">{current.name}</h1>
      </div>
      <ConsoleState state={current.status} />
    </header>
    <dl class="instrument-sheet resource-instruments">
      <div>
        <dt>CPU / HOST</dt>
        <dd>{formatNumber(current.cpuHostPct)}%</dd>
      </div>
      <div>
        <dt>MEMORY</dt>
        <dd>{formatBytes(current.memoryBytes)}</dd>
      </div>
      <div>
        <dt>CONTEXT</dt>
        <dd>
          {current.project ?? current.category ?? 'service'}{current.environment
            ? `/${current.environment}`
            : ''}
        </dd>
      </div>
      <div>
        <dt>COMPONENTS</dt>
        <dd>{current.components?.length ?? 0}</dd>
      </div>
    </dl>
    <section aria-labelledby="components-title">
      <ConsoleSection
        code="UNITS"
        title="Component roster"
        id="components-title"
        detail={`${current.components?.length ?? 0} components`}
      />
      {#if current.components?.length}<table class="console-table">
          <thead
            ><tr><th>State</th><th>Component</th><th>Identity</th></tr></thead
          ><tbody
            >{#each current.components as component (component.id)}<tr
                ><td><ConsoleState state={component.status} /></td><th
                  scope="row">{component.name}</th
                ><td><code>{component.id}</code></td></tr
              >{/each}</tbody
          >
        </table>{:else}<p class="console-empty">
          No component detail is available.
        </p>{/if}
    </section>
    <details class="technical-disclosure">
      <summary>Technical metadata</summary>
      <dl>
        <dt>Resource identity</dt>
        <dd><code>{current.id}</code></dd>
        <dt>Category</dt>
        <dd>{current.category ?? '—'}</dd>
      </dl>
    </details>
  </section>
{:else if archived}
  <section
    class="console-page archived-detail"
    aria-labelledby="resource-title"
  >
    <header class="identity-strip">
      <div>
        <span>ARCHIVE</span>
        <h1 id="resource-title">{archived.name}</h1>
      </div>
      <ConsoleState state="unknown" label="archived" />
    </header>
    <p class="console-notice">
      This workload is no longer active. Historical telemetry remains available
      until explicitly purged.
    </p>
    <dl class="instrument-sheet">
      <div>
        <dt>CONTEXT</dt>
        <dd>
          {archived.project ?? archived.category}{archived.environment
            ? `/${archived.environment}`
            : ''}
        </dd>
      </div>
      <div>
        <dt>ARCHIVED</dt>
        <dd>
          {archived.archivedAt
            ? new Date(archived.archivedAt).toLocaleString()
            : '—'}
        </dd>
      </div>
    </dl>
  </section>
{:else if error}<p class="console-notice" role="alert">{error}</p>{:else}<p
    class="console-empty"
    role="status"
  >
    Loading resource…
  </p>{/if}

{#if current || archived}<HistoryCharts
    scope="resource"
    {id}
    metrics={[
      'cpu',
      'memory',
      'network_rx',
      'network_tx',
      'block_read',
      'block_write',
    ]}
  />{/if}
{#if archived}<HistoryDeletion archivedResourceId={id} />{/if}
