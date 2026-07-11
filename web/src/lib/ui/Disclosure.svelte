<script lang="ts">let { label, children }: { label: string; children?: import('svelte').Snippet } = $props();</script>
<details><summary>{label}</summary><div>{@render children?.()}</div></details>
