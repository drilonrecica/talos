<script lang="ts">
  import type { LiveStore } from './live.svelte';
  import { formatBytes, formatNumber, formatRate } from './i18n';
  import { formatUptime } from './watch';
  import ConsoleSection from './ui/ConsoleSection.svelte';
  import ConsoleState from './ui/ConsoleState.svelte';
  import HistoryCharts from './HistoryCharts.svelte';
  let { live }: { live: LiveStore } = $props();
  let s = $derived(live.snapshot);
  const percent = (used?: number | null, total?: number | null) =>
    used == null || total == null || total <= 0 ? null : (used / total) * 100;
</script>

<section class="console-page" aria-labelledby="server-title">
  <ConsoleSection
    code="HOST"
    title="Instrumentation sheet"
    id="server-title"
    detail={s
      ? `sample ${new Date(s.ts).toLocaleTimeString()}`
      : 'awaiting sample'}
  />
  {#if !s}<p class="console-empty" role="status">
      Loading server telemetry…
    </p>{:else}
    <dl class="instrument-sheet">
      <div>
        <dt>CPU / CURRENT</dt>
        <dd>{formatNumber(s.host.cpuPct)}%</dd>
      </div>
      <div>
        <dt>MEMORY / USED</dt>
        <dd>{formatBytes(s.host.memoryUsedBytes)}</dd>
        <dd class="instrument-note">
          {formatNumber(
            s.host.memoryPct ??
              percent(s.host.memoryUsedBytes, s.host.memoryTotalBytes),
          )}% of {formatBytes(s.host.memoryTotalBytes)}
        </dd>
      </div>
      <div>
        <dt>LOAD / 1 MIN</dt>
        <dd>{formatNumber(s.host.load1)}</dd>
      </div>
      <div>
        <dt>NETWORK / RECEIVE</dt>
        <dd>{formatRate(s.host.networkRxBps)}</dd>
      </div>
      <div>
        <dt>NETWORK / TRANSMIT</dt>
        <dd>{formatRate(s.host.networkTxBps)}</dd>
      </div>
      <div>
        <dt>DISK / USED</dt>
        <dd>{formatBytes(s.host.diskUsedBytes)}</dd>
        <dd class="instrument-note">
          {formatNumber(percent(s.host.diskUsedBytes, s.host.diskTotalBytes))}%
          of {formatBytes(s.host.diskTotalBytes)}
        </dd>
      </div>
      <div>
        <dt>UPTIME</dt>
        <dd>{formatUptime(s.host.uptimeSeconds)}</dd>
      </div>
      <div>
        <dt>BOOT IDENTITY</dt>
        <dd class="technical-value">{s.bootIdentity || 'Unavailable'}</dd>
      </div>
    </dl>
    <section aria-labelledby="collectors-title">
      <ConsoleSection
        code="INPUT"
        title="Collector state"
        id="collectors-title"
        detail={`${Object.keys(s.collectors).length} collectors`}
      />
      <table class="console-table">
        <thead
          ><tr
            ><th>State</th><th>Collector</th><th>Fresh at</th><th>Reason</th
            ></tr
          ></thead
        ><tbody>
          {#each Object.entries(s.collectors) as [name, value]}<tr
              ><td><ConsoleState state={value.state} /></td><th scope="row"
                >{name}</th
              ><td
                >{value.freshAt
                  ? new Date(value.freshAt).toLocaleString()
                  : '—'}</td
              ><td>{value.reason ?? '—'}</td></tr
            >{/each}
        </tbody>
      </table>
    </section>
  {/if}
</section>
<HistoryCharts
  scope="host"
  metrics={['cpu', 'memory', 'network_rx', 'network_tx']}
/>
