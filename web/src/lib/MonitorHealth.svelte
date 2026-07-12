<script lang="ts">
  import { onMount } from 'svelte';
  import Badge from './ui/Badge.svelte';
  import { formatBytes, formatNumber } from './i18n';
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

<section aria-labelledby="monitor-health-title">
  <h1 id="monitor-health-title">Monitor health</h1>
  <p>
    These measurements show Binnacle’s own resource cost and whether history
    work is keeping up. Unavailable values are not treated as zero.
  </p>
  {#if error}<p role="alert">{error}</p>{/if}
  {#if at}<p>
      Updated <time datetime={at}>{new Date(at).toLocaleString()}</time>.
    </p>{/if}
  <div class="monitor-grid" aria-live="polite">
    {#each metrics as metric (metric.id)}
      <article class="card">
        <h2>{metric.label}</h2>
        <p class="monitor-value">{display(metric)}</p>
        <Badge
          state={metric.status === 'normal'
            ? 'healthy'
            : metric.status === 'critical'
              ? 'down'
              : metric.status === 'warning'
                ? 'degraded'
                : 'unknown'}>{metric.status}</Badge
        >
        <p>{metric.help}</p>
        {#if metric.id === 'database' || metric.id === 'wal' || metric.id === 'queue'}<p
          >
            <a href="/settings">Review storage settings and recovery guidance</a
            >
          </p>{/if}
      </article>
    {/each}
  </div>
</section>
