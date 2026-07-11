<script lang="ts">
  import type { LiveStore } from './live.svelte';
  import { formatBytes, formatNumber } from './i18n';
  import Badge from './ui/Badge.svelte';
  import HistoryCharts from './HistoryCharts.svelte';
  let { live }: { live: LiveStore } = $props();
  let s = $derived(live.snapshot);
</script>

{#if !s}<p role="status">Loading server telemetry…</p>{:else}<section
    class="overview"
  >
    <div class="card">
      <h2>CPU</h2>
      <p>{formatNumber(s.host.cpuPct)}%</p>
    </div>
    <div class="card">
      <h2>Memory</h2>
      <p>{formatBytes(s.host.memoryUsedBytes)}</p>
    </div>
    <div class="card">
      <h2>Load</h2>
      <p>{formatNumber(s.host.load1)}</p>
    </div>
    <div class="card">
      <h2>Network</h2>
      <p>RX {formatBytes(s.host.networkRxBps)}/s</p>
      <p>TX {formatBytes(s.host.networkTxBps)}/s</p>
    </div>
    <div class="card">
      <h2>Boot</h2>
      <p>{s.bootIdentity || 'Unavailable'}</p>
    </div>
    <div class="card">
      <h2>Collector health</h2>
      {#each Object.entries(s.collectors) as [name, value]}<p>
          <Badge state={value.state}>{name}: {value.state}</Badge>
        </p>{/each}
    </div>
  </section>{/if}
<HistoryCharts
  scope="host"
  metrics={['cpu', 'memory', 'network_rx', 'network_tx']}
/>
