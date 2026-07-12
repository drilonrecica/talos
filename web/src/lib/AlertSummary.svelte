<script lang="ts">
  import { onMount } from 'svelte';
  let { resourceId = '' }: { resourceId?: string } = $props();
  type Alert = {
    id: string;
    severity: string;
    message: string;
    targetId: string;
  };
  type Check = {
    id: string;
    name: string;
    required: boolean;
    enabled: boolean;
    resourceId: string;
  };
  let alerts = $state<Alert[]>([]),
    checks = $state<Check[]>([]);
  onMount(() => {
    const query = resourceId
      ? `&resource=${encodeURIComponent(resourceId)}`
      : '';
    void Promise.all([
      fetch(`/api/v1/alerts?status=firing${query}`, {
        credentials: 'same-origin',
      }).then((r) => (r.ok ? r.json() : [])),
      resourceId
        ? fetch('/api/v1/checks', { credentials: 'same-origin' }).then((r) =>
            r.ok ? r.json() : [],
          )
        : Promise.resolve([]),
    ]).then(([a, c]) => {
      alerts = a as Alert[];
      checks = (c as Check[]).filter((v) => v.resourceId === resourceId);
    });
  });
</script>

{#if alerts.length || checks.length}<aside
    class="card"
    aria-label={resourceId
      ? 'Resource health checks and alerts'
      : 'Active alert summary'}
  >
    <h2>{resourceId ? 'Checks and related alerts' : 'Active alerts'}</h2>
    {#if alerts.length}<ul>
        {#each alerts.slice(0, 5) as alert (alert.id)}<li>
            <strong>{alert.severity}</strong> — {alert.message}
          </li>{/each}
      </ul>{:else}<p>No active related alerts.</p>{/if}{#if checks.length}<p>
        {checks.filter((c) => c.enabled).length} enabled checks · {checks.filter(
          (c) => c.required,
        ).length} required
      </p>{/if}<a href="/alerts">Open Alerts</a>
  </aside>{/if}
