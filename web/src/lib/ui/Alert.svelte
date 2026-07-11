<script lang="ts">let { level = 'info', children }: { level?: 'info'|'warning'|'error'; children?: import('svelte').Snippet } = $props();</script>
<div role={level === 'error' ? 'alert' : 'status'} data-level={level}>{@render children?.()}</div>
