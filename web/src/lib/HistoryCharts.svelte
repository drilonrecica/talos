<script lang="ts">
  import { onMount } from 'svelte';
  import TimeSeries from './ui/TimeSeries.svelte';
  import {
    fetchHistory,
    chartPoints,
    eventHistoryURL,
    rangeFor,
    validateRange,
    type HistoryResponse,
    type Metric,
    type RangeKey,
  } from './history';
  import { formatBytes, formatNumber, formatRate } from './i18n';
  import {
    boundAnnotations,
    type ChartAnnotation,
    type OperationalEvent,
  } from './annotations';
  import ConsoleSection from './ui/ConsoleSection.svelte';
  import { preferences } from './preferences';

  let {
    scope,
    id,
    metrics,
  }: { scope: 'host' | 'resource'; id?: string; metrics: Metric[] } = $props();
  let range = $state<RangeKey>(preferences().chartRange);
  let from = $state('');
  let to = $state('');
  let data = $state<HistoryResponse | null>(null);
  let error = $state('');
  let loading = $state(false);
  let controller: AbortController | undefined;
  let annotations = $state<OperationalEvent[]>([]);
  let markers = $state<ChartAnnotation[]>([]);
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
  function weightedAverage(
    points: HistoryResponse['series'][number]['points'],
  ) {
    const measured = points.filter(
      (point) => point.avg != null && point.count > 0,
    );
    const count = measured.reduce((sum, point) => sum + point.count, 0);
    return count
      ? measured.reduce(
          (sum, point) => sum + (point.avg ?? 0) * point.count,
          0,
        ) / count
      : null;
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
        eventHistoryURL(scope, id, selected.from, selected.to),
        {
          credentials: 'same-origin',
          signal: controller.signal,
        },
      );
      annotations = events.ok
        ? ((await events.json()) as OperationalEvent[]).filter(
            (event) => event.ts <= selected.to.toISOString(),
          )
        : [];
      markers = boundAnnotations(annotations, selected.from, selected.to);
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
  <ConsoleSection
    code="HISTORY"
    title="Historical telemetry"
    id="history-title"
    detail={data?.resolution ? `resolution ${data.resolution}` : ''}
  />
  <div
    class="control-rail range-controls"
    role="group"
    aria-label="Historical range"
  >
    <span>RANGE</span>
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
  {#if error}<p role="alert">
      {error} <button type="button" onclick={load}>Retry</button>
    </p>{/if}
  {#if data}<div class="history-status-rail">
      <span>Resolution: {data.resolution}.</span><span
        >GAPS {data.gaps.length}</span
      ><span>ANNOTATIONS {annotations.length}</span>
    </div>
    {#if annotations.length}<details class="chart-annotations">
        <summary
          >{annotations.length} event annotation{annotations.length === 1
            ? ''
            : 's'}</summary
        >
        <ul>
          {#each annotations as event (event.ts + event.type)}<li>
              <a
                href={event.resourceId
                  ? `/resources/${event.resourceId}`
                  : '/events'}
                ><time datetime={event.ts}
                  >{new Date(event.ts).toLocaleString()}</time
                >: {event.summary}</a
              >
            </li>{/each}
        </ul>
      </details>{/if}
    {#if data.gaps.length}<details class="chart-gaps">
        <summary
          >{data.gaps.length} data gap{data.gaps.length === 1
            ? ''
            : 's'}</summary
        >
        <ul>
          {#each data.gaps as gap (gap.from + gap.to + gap.reason)}<li>
              <time datetime={gap.from}
                >{new Date(gap.from).toLocaleString()}</time
              >–<time datetime={gap.to}
                >{new Date(gap.to).toLocaleString()}</time
              >: {gap.reason.replaceAll('_', ' ')}
            </li>{/each}
        </ul>
      </details>{/if}
    {#each data.series as series (series.metric)}<article class="metric-band">
        <header>
          <span>METRIC</span>
          <h3>{labels[series.metric]}</h3>
          <strong
            >{display(series.metric, series.points.at(-1)?.avg ?? null)}</strong
          >
        </header>
        <div class="metric-plot">
          <TimeSeries
            label={labels[series.metric]}
            points={chartPoints(series, data.gaps)}
            gaps={data.gaps}
            {markers}
            formatValue={(value) => display(series.metric, value)}
          />
        </div>
        <dl class="metric-stats">
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
          <dt>Average</dt>
          <dd>{display(series.metric, weightedAverage(series.points))}</dd>
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
