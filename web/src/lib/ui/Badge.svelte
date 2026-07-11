<script lang="ts">let { state = 'unknown', children }: { state?: string; children?: import('svelte').Snippet } = $props();</script>
<span class="status" data-state={state}><span aria-hidden="true">●</span>{@render children?.()}</span>
