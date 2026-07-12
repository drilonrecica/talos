<script lang="ts">
  import { onMount } from 'svelte';
  import type { LiveStore } from './live.svelte';
  import Badge from './ui/Badge.svelte';
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
  let showArchived = $state(
    new URLSearchParams(location.search).get('view') === 'archived',
  );
  let resources = $derived(live.snapshot?.resources ?? []);
  onMount(() => {
    if (!showArchived) return;
    void fetch('/api/v1/resources?state=archived', {
      credentials: 'same-origin',
    })
      .then((response) => {
        if (!response.ok)
          throw new Error('Archived resources are unavailable.');
        return response.json() as Promise<Archived[]>;
      })
      .then((values) => (archived = values))
      .catch((reason) => (error = String(reason)));
  });
</script>

<nav class="resource-tabs" aria-label="Resource views">
  <a href="/resources" aria-current={!showArchived ? 'page' : undefined}
    >Active</a
  >
  <a
    href="/resources?view=archived"
    aria-current={showArchived ? 'page' : undefined}>Archived</a
  >
</nav>
{#if showArchived}
  <section>
    <h2>Archived resources</h2>
    <p>
      Archived resources are historical only. Binnacle cannot control or restore
      workloads.
    </p>
    {#if error}<p role="alert">{error}</p>
    {:else if archived.length}{#each archived as resource (resource.id)}<article
          class="archived-resource"
        >
          <a href={`/resources/${resource.id}`}>{resource.name}</a>
          <Badge state="archived">archived</Badge
          >{#if resource.archivedAt}<small>
              Archived {new Date(resource.archivedAt).toLocaleString()}</small
            >{/if}
        </article>{/each}
    {:else}<p>No archived resources.</p>{/if}
  </section>
{:else}
  <section>
    <h2>Active resources</h2>
    {#if resources.length}{#each resources as resource (resource.id)}<article>
          <a href={'/resources/' + resource.id}>{resource.name}</a><Badge
            state={resource.status}>{resource.status}</Badge
          >
        </article>{/each}
    {:else}<p>No active resources.</p>{/if}
  </section>
{/if}
