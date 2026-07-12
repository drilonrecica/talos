<script lang="ts">
  let { archivedResourceId = '' }: { archivedResourceId?: string } = $props();
  type Preview = {
    token: string;
    confirmation: string;
    totalRows: number;
    expiresAt: string;
  };
  let kind = $state('before');
  let resourceId = $state('');
  $effect(() => {
    if (archivedResourceId) {
      kind = 'archived_resource';
      resourceId = archivedResourceId;
    }
  });
  let before = $state('');
  let preview = $state<Preview | null>(null);
  let confirmation = $state('');
  let status = $state('');
  let job = $state<{
    id: string;
    state: string;
    totalRows: number;
    deletedRows: number;
    error?: string;
  } | null>(null);
  async function requestPreview() {
    status = '';
    preview = null;
    const body: Record<string, string> = { kind };
    if (kind === 'before') body.before = new Date(before).toISOString();
    if (kind === 'resource' || kind === 'archived_resource')
      body.resourceId = resourceId;
    const response = await fetch('/api/v1/history/deletion-previews', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf() },
      body: JSON.stringify(body),
    });
    if (!response.ok) {
      status = 'The deletion preview could not be created.';
      return;
    }
    preview = (await response.json()) as Preview;
  }
  async function start() {
    if (!preview) return;
    const response = await fetch('/api/v1/history/deletion-jobs', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf() },
      body: JSON.stringify({ token: preview.token, confirmation }),
    });
    if (!response.ok) {
      status = 'The deletion could not be started.';
      return;
    }
    job = await response.json();
    preview = null;
    status = 'History deletion started.';
    void refresh();
  }
  async function refresh() {
    if (!job) return;
    const response = await fetch(`/api/v1/history/deletion-jobs/${job.id}`, {
      credentials: 'same-origin',
    });
    if (!response.ok) return;
    job = await response.json();
    if (job && ['queued', 'running', 'cancelling'].includes(job.state))
      window.setTimeout(refresh, 1000);
  }
  async function change(action: 'cancel' | 'retry') {
    if (!job) return;
    await fetch(`/api/v1/history/deletion-jobs/${job.id}/${action}`, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'X-CSRF-Token': csrf() },
    });
    void refresh();
  }
  function csrf() {
    return (
      document.cookie
        .split('; ')
        .find((item) => item.startsWith('binnacle_csrf='))
        ?.slice('binnacle_csrf='.length) ?? ''
    );
  }
</script>

<section class="history-deletion" aria-labelledby="history-delete-title">
  <h2 id="history-delete-title">Data history</h2>
  <p>
    History deletion is irreversible. Monitoring configuration and user access
    are preserved.
  </p>
  {#if !archivedResourceId}<label
      >Scope <select bind:value={kind}
        ><option value="before">Delete data before a date</option><option
          value="resource">Delete one resource’s history</option
        ><option value="archived_resource">Purge an archived resource</option
        ><option value="all">Reset all monitoring history</option></select
      ></label
    >{/if}{#if kind === 'before'}<label
      >Before <input
        type="datetime-local"
        bind:value={before}
        required
      /></label
    >{:else if kind !== 'all' && !archivedResourceId}<label
      >Resource ID <input bind:value={resourceId} required /></label
    >{/if}<button type="button" onclick={requestPreview}
    >Preview deletion</button
  >{#if preview}<p>
      {preview.totalRows} rows in the selected {kind} scope will be deleted. Type
      <code>{preview.confirmation}</code> to confirm.
    </p>
    <label>Confirmation <input bind:value={confirmation} /></label><button
      type="button"
      disabled={confirmation !== preview.confirmation}
      onclick={start}>Delete history</button
    >{/if}{#if status}<p role="status">{status}</p>{/if}{#if job}<section
      aria-label="Deletion progress"
    >
      <p role="status">
        {job.state}: {job.deletedRows} of {job.totalRows} rows deleted.
      </p>
      {#if ['queued', 'running'].includes(job.state)}<button
          type="button"
          onclick={() => change('cancel')}>Cancel deletion</button
        >{:else if ['cancelled', 'failed'].includes(job.state)}<button
          type="button"
          onclick={() => change('retry')}>Retry deletion</button
        >{/if}{#if job.error}<p role="alert">{job.error}</p>{/if}
      {#if archivedResourceId && job.state === 'completed'}<p>
          <a href="/resources?view=archived">Return to archived resources</a>
        </p>{/if}
    </section>{/if}
</section>
