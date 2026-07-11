<script lang="ts">let { title, children }: { title: string; children?: import('svelte').Snippet } = $props();</script>
<section aria-label={title}><h2>{title}</h2>{@render children?.()}</section>
