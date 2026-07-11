<script lang="ts">
  import { tick } from 'svelte'; import { t } from '../i18n';
  let { open = $bindable(false), title, children }: { open?: boolean; title: string; children?: import('svelte').Snippet } = $props(); let element: HTMLDialogElement; let opener: HTMLElement | null;
  $effect(() => { if (!element) return; if (open && !element.open) { opener = document.activeElement as HTMLElement; void tick().then(() => element.showModal()); } if (!open && element.open) { element.close(); opener?.focus(); } });
  function close() { open = false; }
</script>
<dialog bind:this={element} aria-labelledby="dialog-title" onclose={close} oncancel={close}><h2 id="dialog-title">{title}</h2>{@render children?.()}<button type="button" onclick={close}>{t('close')}</button></dialog>
