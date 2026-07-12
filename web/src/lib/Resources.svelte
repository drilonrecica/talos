<script lang="ts">
  import { onMount } from 'svelte';
  import type { LiveStore } from './live.svelte';
  import { formatBytes, formatNumber } from './i18n';
  import ConsoleSection from './ui/ConsoleSection.svelte';
  import ConsoleState from './ui/ConsoleState.svelte';
  type Archived = {
    id: string;
    name: string;
    status: string;
    category: string;
    project?: string;
    environment?: string;
    archivedAt?: string;
  };
  let { live }: { live: LiveStore } = $props();
  let archived = $state<Archived[]>([]);
  let error = $state('');
  let loadingArchived = $state(false);
  let showArchived = $state(
    new URLSearchParams(location.search).get('view') === 'archived',
  );
  let resources = $derived(live.snapshot?.resources ?? []);
  onMount(() => {
    if (!showArchived) return;
    loadingArchived = true;
    void fetch('/api/v1/resources?state=archived', {
      credentials: 'same-origin',
    })
      .then((response) => {
        if (!response.ok)
          throw new Error('Archived resources are unavailable.');
        return response.json() as Promise<Archived[]>;
      })
      .then((values) => (archived = values))
      .catch((reason) => (error = String(reason)))
      .finally(() => (loadingArchived = false));
  });
  const context = (resource: Archived | (typeof resources)[number]) =>
    [resource.project ?? resource.category ?? 'service', resource.environment]
      .filter(Boolean)
      .join('/');
</script>

<section class="console-page" aria-labelledby="resources-title">
  <ConsoleSection
    code="ROSTER"
    title="Resources"
    id="resources-title"
    detail={showArchived
      ? `${archived.length} archived`
      : `${resources.length} active`}
  />
  <nav class="control-rail resource-tabs" aria-label="Resource views">
    <span>STATE</span><a
      href="/resources"
      aria-current={!showArchived ? 'page' : undefined}>Active</a
    >
    <a
      href="/resources?view=archived"
      aria-current={showArchived ? 'page' : undefined}>Archived</a
    >
  </nav>
  {#if error}<p class="console-notice" role="alert">{error}</p>
  {:else if loadingArchived}<p class="console-empty" role="status">
      Loading archived resources…
    </p>
  {:else if showArchived && !archived.length}<p class="console-empty">
      No archived resources.
    </p>
  {:else if !showArchived && !resources.length}<p class="console-empty">
      No active resources. Host monitoring remains available.
    </p>
  {:else}
    <div class="table-scroll">
      <table class="console-table resource-roster">
        <thead
          ><tr
            ><th>State</th><th>Resource</th><th>Context</th><th>CPU</th><th
              >Memory</th
            ><th>Components</th>{#if showArchived}<th>Archived</th>{/if}</tr
          ></thead
        >
        <tbody
          >{#each showArchived ? archived : resources as resource (resource.id)}
            <tr data-state={showArchived ? 'archived' : resource.status}>
              <td
                ><ConsoleState
                  state={showArchived ? 'unknown' : resource.status}
                  label={showArchived ? 'archived' : resource.status}
                /></td
              >
              <th scope="row"
                ><a href={`/resources/${resource.id}`}>{resource.name}</a></th
              >
              <td>{context(resource)}</td>
              <td
                >{showArchived
                  ? '—'
                  : `${formatNumber('cpuHostPct' in resource ? resource.cpuHostPct : null)}%`}</td
              >
              <td
                >{showArchived
                  ? '—'
                  : formatBytes(
                      'memoryBytes' in resource ? resource.memoryBytes : null,
                    )}</td
              >
              <td
                >{showArchived
                  ? '—'
                  : 'components' in resource
                    ? (resource.components?.length ?? 0)
                    : '—'}</td
              >
              {#if showArchived}<td
                  >{'archivedAt' in resource && resource.archivedAt
                    ? new Date(resource.archivedAt).toLocaleString()
                    : '—'}</td
                >{/if}
            </tr>
          {/each}</tbody
        >
      </table>
    </div>
    {#if showArchived}<p class="console-caption">
        Archived resources are historical only. Binnacle cannot control or
        restore workloads.
      </p>{/if}
  {/if}
</section>
