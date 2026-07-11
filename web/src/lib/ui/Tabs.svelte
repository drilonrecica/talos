<script lang="ts">let { tabs, active = $bindable() }: { tabs: { id:string; label:string }[]; active: string } = $props();</script>
<div role="tablist">{#each tabs as tab}<button role="tab" aria-selected={active === tab.id} onclick={() => active = tab.id}>{tab.label}</button>{/each}</div>
