<script lang="ts">
  import type { LiveStore, LiveSnapshot } from './live.svelte';
  import { formatBytes, formatNumber, formatRate } from './i18n';
  import PostSetupChecklist from './PostSetupChecklist.svelte';
  import AlertSummary from './AlertSummary.svelte';
  import {
    formatUptime,
    meterValue,
    prioritizedResources,
    staleResource,
  } from './watch';

  let {
    live,
    inspectID,
    oninspect,
  }: {
    live: LiveStore;
    inspectID: string;
    oninspect: (id: string | null) => void;
  } = $props();

  let snapshot = $derived(live.snapshot);
  let resources = $derived(
    snapshot
      ? prioritizedResources(
          snapshot.resources.filter((resource) => !resource.infrastructure),
        )
      : [],
  );
  let infrastructure = $derived(
    snapshot?.resources.filter((resource) => resource.infrastructure) ?? [],
  );
  let selected = $derived(
    inspectID
      ? snapshot?.resources.find((resource) => resource.id === inspectID)
      : undefined,
  );
  let memoryPercent = $derived(
    snapshot
      ? (snapshot.host.memoryPct ??
          ratio(snapshot.host.memoryUsedBytes, snapshot.host.memoryTotalBytes))
      : null,
  );
  let diskPercent = $derived(
    snapshot
      ? ratio(snapshot.host.diskUsedBytes, snapshot.host.diskTotalBytes)
      : null,
  );
  let attention = $derived(
    snapshot
      ? resources.filter(
          (resource) =>
            resource.status !== 'healthy' ||
            staleResource(resource, snapshot.ts),
        )
      : [],
  );
  let collectorProblems = $derived(
    snapshot
      ? Object.entries(snapshot.collectors).filter(
          ([, collector]) => collector.state !== 'healthy',
        )
      : [],
  );

  function ratio(used?: number | null, total?: number | null) {
    if (used == null || total == null || total <= 0) return null;
    return (used / total) * 100;
  }

  function value(value: number | null | undefined, suffix = '') {
    return value == null || !Number.isFinite(value)
      ? '—'
      : `${formatNumber(value)}${suffix}`;
  }

  function inspect(event: MouseEvent, id: string) {
    if (
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    )
      return;
    event.preventDefault();
    oninspect(id);
  }

  function isStale(resource: LiveSnapshot['resources'][number]) {
    return snapshot ? staleResource(resource, snapshot.ts) : true;
  }
</script>

{#if !snapshot}
  <section class="watch-loading" aria-live="polite">
    <span aria-hidden="true">WAIT</span>
    <p>Awaiting current telemetry…</p>
  </section>
{:else}
  <AlertSummary />
  <section class="host-band" aria-labelledby="host-band-title">
    <div class="band-heading">
      <span>HOST</span>
      <h1 id="host-band-title">Watch</h1>
    </div>
    <dl class="host-instruments">
      <div>
        <dt>CPU</dt>
        <dd class="meter-reading">
          <span>{value(snapshot.host.cpuPct, '%')}</span>
          <meter
            aria-label="Host CPU utilization"
            min="0"
            max="100"
            value={meterValue(snapshot.host.cpuPct) ?? 0}
            data-unavailable={meterValue(snapshot.host.cpuPct) == null}
          ></meter>
        </dd>
      </div>
      <div>
        <dt>RAM</dt>
        <dd class="meter-reading">
          <span>{formatBytes(snapshot.host.memoryUsedBytes)}</span>
          <meter
            aria-label="Host memory utilization"
            min="0"
            max="100"
            value={meterValue(memoryPercent) ?? 0}
            data-unavailable={meterValue(memoryPercent) == null}
          ></meter>
        </dd>
      </div>
      <div>
        <dt>DISK</dt>
        <dd class="meter-reading">
          <span>{formatBytes(snapshot.host.diskUsedBytes)}</span>
          <meter
            aria-label="Host disk utilization"
            min="0"
            max="100"
            value={meterValue(diskPercent) ?? 0}
            data-unavailable={meterValue(diskPercent) == null}
          ></meter>
        </dd>
      </div>
      <div class="instrument-compact">
        <dt>LOAD</dt>
        <dd>{value(snapshot.host.load1)}</dd>
      </div>
      <div class="instrument-network">
        <dt>NETWORK</dt>
        <dd>
          <span>↓ {formatRate(snapshot.host.networkRxBps)}</span><span
            >↑ {formatRate(snapshot.host.networkTxBps)}</span
          >
        </dd>
      </div>
      <div class="instrument-compact">
        <dt>UPTIME</dt>
        <dd>{formatUptime(snapshot.host.uptimeSeconds)}</dd>
      </div>
    </dl>
  </section>

  <div class="watch-workspace" class:inspecting={Boolean(inspectID)}>
    <section class="resource-watch" aria-labelledby="resource-watch-title">
      <header class="section-line">
        <div>
          <span>ROSTER</span>
          <h2 id="resource-watch-title">Active services</h2>
        </div>
        <output>{resources.length} tracked</output>
      </header>
      {#if resources.length}
        <div class="roster-scroll">
          <table class="resource-roster">
            <thead>
              <tr>
                <th scope="col">State</th>
                <th scope="col">Service</th>
                <th scope="col">Context</th>
                <th scope="col">CPU</th>
                <th scope="col">Memory</th>
                <th scope="col">Units</th>
              </tr>
            </thead>
            <tbody>
              {#each resources as resource (resource.id)}
                {@const stale = isStale(resource)}
                <tr
                  class:selected={inspectID === resource.id}
                  class:stale
                  data-state={stale ? 'unknown' : resource.status}
                >
                  <td>
                    <span
                      class="roster-state"
                      data-state={stale ? 'unknown' : resource.status}
                    >
                      <span aria-hidden="true"
                        >{stale
                          ? '◇'
                          : resource.status === 'healthy'
                            ? '●'
                            : '▲'}</span
                      >
                      {stale ? 'stale' : resource.status}
                    </span>
                  </td>
                  <th scope="row">
                    <a
                      href={`/watch?inspect=${encodeURIComponent(resource.id)}`}
                      onclick={(event) => inspect(event, resource.id)}
                      >{resource.name}</a
                    >
                  </th>
                  <td class="resource-context">
                    {resource.project ??
                      resource.category ??
                      'service'}{#if resource.environment}<span
                        >/{resource.environment}</span
                      >{/if}
                  </td>
                  <td>{stale ? '—' : value(resource.cpuHostPct, '%')}</td>
                  <td>{stale ? '—' : formatBytes(resource.memoryBytes)}</td>
                  <td>{resource.components?.length ?? 0}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {:else}
        <div class="watch-empty">
          <strong>NO ACTIVE SERVICES</strong>
          <p>Host monitoring remains available.</p>
        </div>
      {/if}
      {#if infrastructure.length}
        <footer class="infrastructure-line">
          <span>INFRA</span>
          {#each infrastructure as resource (resource.id)}
            <a href={`/resources/${resource.id}`}>{resource.name}</a>
            <span class="roster-state" data-state={resource.status}
              >{resource.status}</span
            >
          {/each}
        </footer>
      {/if}
    </section>

    <aside class="watch-rail" aria-live="polite">
      {#if inspectID}
        <header class="section-line inspector-heading">
          <div>
            <span>INSPECT</span>
            <h2>{selected?.name ?? 'Unavailable resource'}</h2>
          </div>
          <button
            type="button"
            onclick={() => oninspect(null)}
            aria-label="Close resource inspector">×</button
          >
        </header>
        {#if selected}
          {@const stale = isStale(selected)}
          <div class="inspector-status">
            <span
              class="roster-state"
              data-state={stale ? 'unknown' : selected.status}
            >
              <span aria-hidden="true"
                >{stale ? '◇' : selected.status === 'healthy' ? '●' : '▲'}</span
              >
              {stale ? 'stale' : selected.status}
            </span>
            <span>{selected.category ?? 'service'}</span>
          </div>
          <dl class="inspector-values">
            <div>
              <dt>CPU / HOST</dt>
              <dd>{stale ? '—' : value(selected.cpuHostPct, '%')}</dd>
            </div>
            <div>
              <dt>MEMORY</dt>
              <dd>{stale ? '—' : formatBytes(selected.memoryBytes)}</dd>
            </div>
            <div>
              <dt>PROJECT</dt>
              <dd>{selected.project ?? '—'}</dd>
            </div>
            <div>
              <dt>ENVIRONMENT</dt>
              <dd>{selected.environment ?? '—'}</dd>
            </div>
          </dl>
          {#if selected.components?.length}
            <section
              class="component-list"
              aria-labelledby="component-list-title"
            >
              <h3 id="component-list-title">
                Components / {selected.components.length}
              </h3>
              <ul>
                {#each selected.components as component (component.id)}
                  <li>
                    <span class="roster-state" data-state={component.status}
                      >{component.status}</span
                    ><code>{component.name}</code>
                  </li>
                {/each}
              </ul>
            </section>
          {/if}
          <div class="technical-id">
            <span>ID</span><code>{selected.id}</code>
          </div>
          <a class="inspector-link" href={`/resources/${selected.id}`}
            >Open full record →</a
          >
        {:else}
          <div class="watch-empty">
            <strong>RESOURCE NOT ON WATCH</strong>
            <p>It may have stopped or been archived since this view opened.</p>
            <button type="button" onclick={() => oninspect(null)}
              >Return to roster</button
            >
          </div>
        {/if}
      {:else}
        <section
          class="rail-section attention-list"
          aria-labelledby="attention-title"
        >
          <header class="section-line">
            <div>
              <span>EXCEPTIONS</span>
              <h2 id="attention-title">Attention</h2>
            </div>
            <output>{attention.length + collectorProblems.length}</output>
          </header>
          {#if collectorProblems.length}
            {#each collectorProblems as [name, collector]}
              <p class="attention-item" data-state={collector.state}>
                <strong>{name} collector</strong>
                <span>{collector.reason ?? collector.state}</span>
              </p>
            {/each}
          {/if}
          {#if attention.length}
            {#each attention.slice(0, 6) as resource (resource.id)}
              <a
                class="attention-item"
                data-state={isStale(resource) ? 'unknown' : resource.status}
                href={`/watch?inspect=${encodeURIComponent(resource.id)}`}
                onclick={(event) => inspect(event, resource.id)}
                ><strong>{resource.name}</strong><span
                  >{isStale(resource)
                    ? 'stale telemetry'
                    : resource.status}</span
                ></a
              >
            {/each}
          {/if}
          {#if !attention.length && !collectorProblems.length}
            <p class="all-clear">
              <span aria-hidden="true">●</span> All watched systems nominal
            </p>
          {/if}
        </section>

        <PostSetupChecklist />

        <section class="rail-section logbook" aria-labelledby="logbook-title">
          <header class="section-line">
            <div>
              <span>LIVE FEED</span>
              <h2 id="logbook-title">Logbook</h2>
            </div>
          </header>
          {#if live.events.length}
            <ol>
              {#each live.events.slice(-8).reverse() as event (event.id)}
                <li>
                  <span>{String(event.id).padStart(4, '0')}</span>
                  <a
                    href={event.resourceId
                      ? `/resources/${event.resourceId}`
                      : '/events'}>{event.message}</a
                  >
                </li>
              {/each}
            </ol>
          {:else}
            <p class="quiet">No recent state changes.</p>
          {/if}
        </section>
      {/if}
    </aside>
  </div>
{/if}
