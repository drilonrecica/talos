<script lang="ts">
  import type { LiveStore } from './live.svelte';
  import Badge from './ui/Badge.svelte';
  import { formatBytes, formatNumber } from './i18n';
  import HistoryCharts from './HistoryCharts.svelte';
  let { live, id }: { live: LiveStore; id: string } = $props();
  let resource = $derived(live.snapshot?.resources.find((v) => v.id === id));
</script>

{#if resource}<section class="card">
    <h2>{resource.name}</h2>
    <Badge state={resource.status}>{resource.status}</Badge>
    <p>CPU: {formatNumber(resource.cpuHostPct)}% of host</p>
    <p>Memory: {formatBytes(resource.memoryBytes)}</p>
    <details>
      <summary>Technical details</summary><code>{resource.id}</code>
    </details>
  </section>
  <HistoryCharts
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
  />{:else}<p>Resource unavailable.</p>{/if}
