<script lang="ts">
  import { onMount } from 'svelte';
  import TimeSeries from './ui/TimeSeries.svelte';
  import {
    fetchHistory,
    chartPoints,
    rangeFor,
    validateRange,
    type HistoryResponse,
    type Metric,
    type RangeKey,
  } from './history';
  import { formatBytes, formatNumber, formatRate } from './i18n';

  let {
    scope,
    id,
    metrics,
  }: { scope: 'host' | 'resource'; id?: string; metrics: Metric[] } = $props();
  let range = $state<RangeKey>('24h');
  let from = $state('');
  let to = $state('');
  let data = $state<HistoryResponse | null>(null);
  let error = $state('');
  let loading = $state(false);
  let controller: AbortController | undefined;
  let annotations = $state<
    Array<{ ts: string; type: string; summary: string }>
  >([]);
  const labels: Record<Metric, string> = {
    cpu: 'CPU (host-normalized %)',
    memory: 'Memory',
    network_rx: 'Network receive',
    network_tx: 'Network transmit',
    block_read: 'Block read',
    block_write: 'Block write',
  };
  function display(metric: Metric, value: number | null) {
    if (metric === 'cpu')
      return value == null ? 'Unavailable' : `${formatNumber(value)}%`;
    if (metric === 'memory') return formatBytes(value);
    return formatRate(value);
  }
  async function load() {
    controller?.abort();
    controller = new AbortController();
    error = '';
    loading = true;
    const selected =
      range === 'custom'
        ? { from: new Date(from), to: new Date(to) }
        : rangeFor(range);
    const problem = validateRange(selected.from, selected.to);
    if (problem) {
      error = problem;
      loading = false;
      return;
    }
    try {
      data = await fetchHistory(
        scope,
        id,
        metrics,
        selected.from,
        selected.to,
        controller.signal,
      );
      const events = await fetch(
        `/api/v1/events?from=${encodeURIComponent(selected.from.toISOString())}`,
        { credentials: 'same-origin', signal: controller.signal },
      );
      annotations = events.ok
        ? (
            (await events.json()) as Array<{
              ts: string;
              type: string;
              summary: string;
            }>
          ).filter((event) => event.ts <= selected.to.toISOString())
        : [];
    } catch (e) {
      if ((e as Error).name !== 'AbortError') error = (e as Error).message;
    } finally {
      loading = false;
    }
  }
  function select(value: RangeKey) {
    range = value;
    if (value !== 'custom') void load();
  }
  function applyCustom() {
    range = 'custom';
    void load();
  }
  onMount(() => {
    void load();
    return () => controller?.abort();
  });
</script>

<section class="history" aria-labelledby="history-title">
  <h2 id="history-title">Historical telemetry</h2>
  <div class="range-controls" role="group" aria-label="Historical range">
    {#each ['1h', '6h', '24h', '7d', '30d'] as item}<button
        type="button"
        aria-pressed={range === item}
        onclick={() => select(item as RangeKey)}>{item}</button
      >{/each}
    <button
      type="button"
      aria-pressed={range === 'custom'}
      onclick={() => {
        range = 'custom';
      }}>Custom</button
    >
  </div>
  {#if range === 'custom'}<form
      onsubmit={(event) => {
        event.preventDefault();
        applyCustom();
      }}
    >
      <label
        >From <input required type="datetime-local" bind:value={from} /></label
      ><label>To <input required type="datetime-local" bind:value={to} /></label
      ><button type="submit">Apply range</button>
    </form>{/if}
  {#if loading}<p role="status">Loading historical telemetry…</p>{/if}
  {#if error}<p role="alert">{error}</p>{/if}
  {#if data}<p class="resolution">
      Resolution: {data.resolution}. Gaps are shown as broken lines.
    </p>
    {#if annotations.length}<details class="chart-annotations">
        <summary
          >{annotations.length} event annotation{annotations.length === 1
            ? ''
            : 's'}</summary
        >
        <ul>
          {#each annotations as event (event.ts + event.type)}<li>
              <time datetime={event.ts}
                >{new Date(event.ts).toLocaleString()}</time
              >: {event.summary}
            </li>{/each}
        </ul>
      </details>{/if}
    {#each data.series as series (series.metric)}<article class="card">
        <h3>{labels[series.metric]}</h3>
        <TimeSeries
          label={labels[series.metric]}
          points={chartPoints(series, data.gaps)}
          gaps={data.gaps}
          markers={annotations.map((event) => ({
            at: new Date(event.ts).getTime() / 1000,
            label: event.summary,
          }))}
        />
        <dl>
          <dt>Current</dt>
          <dd>{display(series.metric, series.points.at(-1)?.avg ?? null)}</dd>
          <dt>Minimum</dt>
          <dd>
            {display(
              series.metric,
              series.points.reduce<number | null>(
                (v, point) =>
                  v == null || (point.min != null && point.min < v)
                    ? point.min
                    : v,
                null,
              ),
            )}
          </dd>
          <dt>Maximum</dt>
          <dd>
            {display(
              series.metric,
              series.points.reduce<number | null>(
                (v, point) =>
                  v == null || (point.max != null && point.max > v)
                    ? point.max
                    : v,
                null,
              ),
            )}
          </dd>
        </dl>
      </article>{/each}{:else if !loading && !error}<p>
      No historical measurements are available for this range.
    </p>{/if}
</section>
