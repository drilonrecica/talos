<script lang="ts">
  import { onMount } from 'svelte';
  import { formatBytes, formatNumber } from './i18n';
  import ConsoleSection from './ui/ConsoleSection.svelte';
  import ConsoleState from './ui/ConsoleState.svelte';
  type Metric = {
    id: string;
    label: string;
    value: number | string | null;
    unit?: string;
    status: string;
    help: string;
  };
  let metrics = $state<Metric[]>([]);
  let at = $state('');
  let error = $state('');
  let timer: number | undefined;
  function display(metric: Metric) {
    if (metric.value == null) return 'Unavailable';
    if (metric.unit === 'bytes') return formatBytes(Number(metric.value));
    if (metric.unit === 'percent')
      return `${formatNumber(Number(metric.value))}%`;
    if (metric.unit === 'milliseconds')
      return `${formatNumber(Number(metric.value))} ms`;
    return `${metric.value}${metric.unit ? ` ${metric.unit}` : ''}`;
  }
  async function load() {
    try {
      const response = await fetch('/api/v1/monitor-health', {
        credentials: 'same-origin',
      });
      if (!response.ok) throw new Error('Monitor health is unavailable.');
      const value = (await response.json()) as {
        at: string;
        metrics: Metric[];
      };
      metrics = value.metrics;
      at = value.at;
      error = '';
    } catch (reason) {
      error =
        reason instanceof Error ? reason.message : 'Monitor health failed.';
    } finally {
      timer = window.setTimeout(load, 5000);
    }
  }
  onMount(() => {
    void load();
    return () => window.clearTimeout(timer);
  });
</script>

<section class="console-page" aria-labelledby="monitor-health-title">
  <ConsoleSection
    code="SELF"
    title="Monitor health"
    id="monitor-health-title"
    detail={at ? `updated ${new Date(at).toLocaleTimeString()}` : ''}
  />
  <p class="console-caption">
    Binnacle resource cost and history pipeline state. Unavailable values are
    not treated as zero.
  </p>
  {#if error}<p class="console-notice" role="alert">{error}</p>{/if}
  <div class="table-scroll" aria-live="polite">
    <table class="console-table health-table">
      <thead
        ><tr
          ><th>State</th><th>Metric</th><th>Reading</th><th>Interpretation</th
          ></tr
        ></thead
      ><tbody
        >{#each [...metrics].sort( (a, b) => a.status.localeCompare(b.status) ) as metric (metric.id)}<tr
            ><td><ConsoleState state={metric.status} /></td><th scope="row"
              >{metric.label}</th
            ><td class="monitor-value">{display(metric)}</td><td
              >{metric.help}{#if ['database', 'wal', 'queue'].includes(metric.id)}
                <a href="/settings#retention">Review storage settings</a
                >{/if}</td
            ></tr
          >{/each}</tbody
      >
    </table>
  </div>
</section>
