<script lang="ts">
  import type { LiveStore } from './live.svelte';
  import Badge from './ui/Badge.svelte';
  import { formatBytes, formatNumber } from './i18n';
  let { live }: { live: LiveStore } = $props();
  let snapshot = $derived(live.snapshot);
</script>

{#if !snapshot}<p role="status">Loading current telemetry…</p>
{:else}<section class="overview">
    <div class="card">
      <h2>Server health</h2>
      <p>
        <Badge state={live.state === 'connected' ? 'healthy' : 'unknown'}
          >{live.state}</Badge
        >
      </p>
      <dl>
        <dt>CPU</dt>
        <dd>{formatNumber(snapshot.host.cpuPct)}%</dd>
        <dt>Memory</dt>
        <dd>{formatBytes(snapshot.host.memoryUsedBytes)}</dd>
      </dl>
    </div>
    <div class="card">
      <h2>Resources</h2>
      {#if snapshot.resources.length}{#each snapshot.resources as resource}<article
          >
            <h3>{resource.name}</h3>
            <Badge state={resource.status}>{resource.status}</Badge>
            <p>
              CPU {formatNumber(resource.cpuHostPct)}% · Memory {formatBytes(
                resource.memoryBytes,
              )}
            </p>
          </article>{/each}{:else}<p>No active resources.</p>{/if}
    </div>
    <div class="card">
      <h2>Collectors</h2>
      {#each Object.entries(snapshot.collectors) as [name, collector]}<p>
          <Badge state={collector.state}>{name}: {collector.state}</Badge
          >{#if collector.reason}
            — {collector.reason}{/if}
        </p>{/each}
    </div>
    <div class="card">
      <h2>Recent events</h2>
      {#if live.events.length}{#each live.events.slice(-5).reverse() as event}<p
          >
            {event.message}
          </p>{/each}{:else}<p>No recent events.</p>{/if}
    </div>
  </section>{/if}
