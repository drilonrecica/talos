<script lang="ts">let { type = 'button', disabled = false, children, ...rest }: { type?: 'button' | 'submit' | 'reset'; disabled?: boolean; children?: import('svelte').Snippet; [key: string]: unknown } = $props();</script>
<button {type} {disabled} {...rest}>{@render children?.()}</button>
