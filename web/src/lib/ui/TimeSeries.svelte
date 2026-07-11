<script lang="ts">
  import { onMount } from 'svelte';
  import uPlot from 'uplot';
  import 'uplot/dist/uPlot.min.css';
  import { summary, toSeries, type Point } from '../chart';
  type Gap = { from: string; to: string; reason: string };
  type Marker = { at: number; label: string };
  let {
    points,
    label,
    variant = 'line',
    gaps = [],
    markers = [],
  }: {
    points: Point[];
    label: string;
    variant?: 'line' | 'area' | 'sparkline';
    gaps?: Gap[];
    markers?: Marker[];
  } = $props();
  let root: HTMLDivElement;
  let plot: uPlot | undefined;
  let selected = $state(0);
  function options(): uPlot.Options {
    return {
      width: root.clientWidth || 1,
      height: variant === 'sparkline' ? 48 : 180,
      series: [
        {},
        {
          label,
          stroke: 'var(--chart-1)',
          fill: variant === 'area' ? 'rgb(120 220 232 / .2)' : undefined,
        },
      ],
      axes: variant === 'sparkline' ? [] : [{}, {}],
      plugins: [
        {
          hooks: {
            draw: [
              (u) => {
                const ctx = u.ctx;
                ctx.save();
                ctx.strokeStyle = 'rgba(245,196,81,.8)';
                for (const marker of markers) {
                  const x = Math.round(u.valToPos(marker.at, 'x', true));
                  ctx.beginPath();
                  ctx.moveTo(x, u.bbox.top);
                  ctx.lineTo(x, u.bbox.top + u.bbox.height);
                  ctx.stroke();
                }
                ctx.restore();
              },
            ],
          },
        },
      ],
    };
  }
  onMount(() => {
    const resize = new ResizeObserver(() =>
      plot?.setSize({ width: root.clientWidth || 1, height: plot!.height }),
    );
    plot = new uPlot(options(), toSeries(points), root);
    return () => {
      resize.disconnect();
      plot?.destroy();
    };
  });
  $effect(() => {
    if (plot) plot.setData(toSeries(points));
  });
  let stats = $derived(summary(points));
  function inspect(event: KeyboardEvent) {
    if (!points.length) return;
    if (event.key === 'ArrowLeft') {
      selected = Math.max(0, selected - 1);
      event.preventDefault();
    }
    if (event.key === 'ArrowRight') {
      selected = Math.min(points.length - 1, selected + 1);
      event.preventDefault();
    }
  }
</script>

<div bind:this={root} aria-hidden="true"></div>
<button
  type="button"
  class="chart-inspector"
  aria-label={`${label} chart inspection`}
  onkeydown={inspect}
  ><span
    >{label}: {#if stats}minimum {stats.min}, average {stats.avg.toFixed(1)},
      maximum {stats.max}{:else}no measurements{/if}</span
  >{#if points[selected]}<span role="status"
      >Selected point: {new Date(points[selected].at * 1000).toLocaleString()}, {points[
        selected
      ].value ?? 'gap'}</span
    >{/if}{#if gaps.length}<span
      >{gaps.length} explicit data gap{gaps.length === 1 ? '' : 's'}.</span
    >{/if}</button
>
{#if markers.length}<ul class="sr-only" aria-label="Chart event annotations">
    {#each markers as marker (marker.at + marker.label)}<li>
        {new Date(marker.at * 1000).toLocaleString()}: {marker.label}
      </li>{/each}
  </ul>{/if}
