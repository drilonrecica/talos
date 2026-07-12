<script lang="ts">
  let { state, label = state }: { state: string; label?: string } = $props();
  const normalized = $derived(
    ['healthy', 'normal', 'passed'].includes(state)
      ? 'healthy'
      : ['warning', 'degraded'].includes(state)
        ? 'degraded'
        : ['down', 'error', 'critical', 'failed'].includes(state)
          ? 'down'
          : 'unknown',
  );
</script>

<span class="roster-state" data-state={normalized}
  ><span aria-hidden="true"
    >{normalized === 'healthy'
      ? '●'
      : normalized === 'unknown'
        ? '◇'
        : '▲'}</span
  >{label}</span
>
