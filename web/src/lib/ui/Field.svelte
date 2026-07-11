<script lang="ts">let { label, error, children }: { label: string; error?: string; children?: import('svelte').Snippet } = $props();</script>
<label>{label}{@render children?.()}{#if error}<span role="alert">{error}</span>{/if}</label>
