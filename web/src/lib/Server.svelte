<script lang="ts">
  import { onMount } from 'svelte';
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
  type Process = {
    pid: number;
    command: string;
    cpuPct: number;
    rssBytes: number;
    user: string;
    state: string;
    uptimeSeconds: number;
    containerId?: string;
  };
  let processes = $state<Process[]>([]);
  let processError = $state('');
  let processLoading = $state(false);
  async function loadProcesses() {
    processLoading = true;
    processError = '';
    try {
      const response = await fetch('/api/v1/processes?limit=25');
      if (!response.ok) throw new Error('Process sample unavailable.');
      processes = (await response.json()).processes;
    } catch (error) {
      processError =
        error instanceof Error ? error.message : 'Process sample unavailable.';
    } finally {
      processLoading = false;
    }
  }
  onMount(() => {
    void loadProcesses();
  });
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
<section class="console-page" aria-labelledby="process-title">
  <ConsoleSection
    code="PROC"
    title="Top host processes"
    id="process-title"
    detail="two-sample / on demand"
  />
  <div class="control-rail">
    <button onclick={() => loadProcesses()} disabled={processLoading}
      >Refresh sample</button
    >
  </div>
  {#if processError}<p role="status">{processError}</p>{/if}
  <div class="table-scroll">
    <table class="console-table">
      <thead
        ><tr
          ><th>PID</th><th>Command</th><th>CPU</th><th>RSS</th><th>User</th><th
            >State</th
          ><th>Uptime</th><th>Container</th></tr
        ></thead
      >
      <tbody
        >{#each processes as process (process.pid)}<tr
            ><td>{process.pid}</td><td class="technical-value"
              >{process.command}</td
            ><td>{formatNumber(process.cpuPct)}%</td><td
              >{formatBytes(process.rssBytes)}</td
            ><td>{process.user}</td><td>{process.state}</td><td
              >{formatUptime(process.uptimeSeconds)}</td
            ><td>{process.containerId?.slice(0, 12) ?? '—'}</td></tr
          >{/each}</tbody
      >
    </table>
  </div>
</section>
