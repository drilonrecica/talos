<script lang="ts">
  interface Preview {
    id: string;
    createdAt: string;
    expiresAt: string;
    fields: Record<string, unknown>;
    partialFailures?: string[];
  }
  let preview = $state<Preview | null>(null);
  let busy = $state(false);
  let error = $state('');

  async function generate() {
    busy = true;
    error = '';
    try {
      const response = await fetch('/api/v1/diagnostics/previews', {
        method: 'POST',
        credentials: 'same-origin',
      });
      if (!response.ok)
        throw new Error(
          response.status === 429
            ? 'Diagnostics generation is rate limited. Try again later.'
            : 'Diagnostics could not be generated.',
        );
      preview = (await response.json()) as Preview;
    } catch (reason) {
      error = reason instanceof Error ? reason.message : 'Diagnostics failed.';
    } finally {
      busy = false;
    }
  }
</script>

<section aria-labelledby="diagnostics-bundle-title">
  <h1 id="diagnostics-bundle-title">Diagnostics bundle</h1>
  <p>
    Generate a sanitized preview first. Passwords, tokens, environment
    variables, application logs, domains, IP addresses, and database contents
    are excluded.
  </p>
  {#if error}<p role="alert">{error}</p>{/if}
  <button type="button" disabled={busy} onclick={generate}
    >{busy ? 'Generating…' : 'Generate preview'}</button
  >
  {#if preview}
    <h2>Exact included fields</h2>
    <dl>
      {#each Object.entries(preview.fields) as [name, value] (name)}
        <dt>{name}</dt>
        <dd><pre>{JSON.stringify(value, null, 2)}</pre></dd>
      {/each}
    </dl>
    {#if preview.partialFailures?.length}<h2>Partial collection failures</h2>
      <ul>
        {#each preview.partialFailures as failure}<li>{failure}</li>{/each}
      </ul>{/if}
    <p>Preview expires {new Date(preview.expiresAt).toLocaleString()}.</p>
    <a
      class="button"
      href={`/api/v1/diagnostics/previews/${preview.id}/download`}
      >Download reviewed bundle</a
    >
  {/if}
</section>
