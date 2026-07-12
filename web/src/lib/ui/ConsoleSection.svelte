<script lang="ts">
  let {
    code,
    title,
    id,
    detail = '',
  }: { code: string; title: string; id: string; detail?: string } = $props();
</script>

<header class="section-line console-section">
  <div>
    <span>{code}</span>
    <h2 {id}>{title}</h2>
  </div>
  {#if detail}<output>{detail}</output>{/if}
</header>
