<script lang="ts">
  import type { LiveStore } from './live.svelte';
  let { live }: { live: LiveStore } = $props();
</script>

<section>
  <h2>Events</h2>
  {#if live.events.length}{#each live.events.slice().reverse() as event}<details
      >
        <summary>{event.type}: {event.message}</summary><code>{event.id}</code>
      </details>{/each}{:else}<p>No events yet.</p>{/if}
</section>
