<script lang="ts">
  import { onMount } from 'svelte';
  import HistoryDeletion from './HistoryDeletion.svelte';
  import {
    applyPreferences,
    preferences,
    type Density,
    type Theme,
  } from './preferences';
  import {
    aggressiveInterval,
    loadSettings,
    patchSetting,
    type SettingsSnapshot,
  } from './settings';
  let snapshot = $state<SettingsSnapshot | null>(null);
  let error = $state('');
  let busy = $state('');
  let theme = $state<Theme>('system');
  let density = $state<Density>('comfortable');
  onMount(() => {
    ({ theme, density } = preferences());
    void loadSettings()
      .then((value) => (snapshot = value))
      .catch((reason) => (error = String(reason)));
  });

  async function save(key: string, value: string) {
    if (!snapshot) return;
    busy = key;
    error = '';
    try {
      snapshot = await patchSetting(snapshot.revision, key, value);
    } catch (reason) {
      error =
        reason instanceof Error
          ? reason.message
          : 'Setting could not be saved.';
    } finally {
      busy = '';
    }
  }
  function appearance(nextTheme: Theme, nextDensity: Density) {
    theme = nextTheme;
    density = nextDensity;
    applyPreferences({ theme, density });
  }
</script>

{#if error}<p role="alert">{error}</p>{/if}
{#if !snapshot}<p role="status">Loading settings…</p>
{:else}
  <div class="settings-layout">
    <nav class="settings-index" aria-label="Settings sections">
      <span>INDEX</span><a href="#collection">01 Collection</a><a
        href="#retention">02 Retention</a
      ><a href="#authentication">03 Authentication</a><a href="#appearance"
        >04 Appearance</a
      ><a href="#privacy">05 Privacy</a><a href="#system">06 System</a><a
        href="#history">07 History</a
      >
    </nav>
    <div class="settings-ledger">
      <section
        id="collection"
        class="ledger-section"
        aria-labelledby="collection-settings"
      >
        <h2 id="collection-settings"><span>01</span> Collection</h2>
        {#each ['collection.host_interval', 'collection.container_interval'] as key}
          {@const setting = snapshot.values[key]}
          <label for={key}
            >{key.endsWith('host_interval')
              ? 'Host interval'
              : 'Container interval'}</label
          >
          <input
            id={key}
            value={setting.value}
            disabled={busy === key}
            onblur={(event) => save(key, event.currentTarget.value)}
          />
          <small
            ><span>{setting.source}</span><span
              >{setting.applyMode === 'live'
                ? 'Applies live'
                : 'Restart required'}</span
            ></small
          >
          {#if aggressiveInterval(setting.value)}<p class="warning">
              Intervals below 2 seconds increase CPU and Docker API load.
            </p>{/if}
        {/each}
        <label for="persistence.raw_interval">Persistence interval</label>
        <input
          id="persistence.raw_interval"
          value={snapshot.values['persistence.raw_interval'].value}
          onblur={(event) =>
            save('persistence.raw_interval', event.currentTarget.value)}
        />
      </section>

      <section
        id="retention"
        class="ledger-section"
        aria-labelledby="retention-settings"
      >
        <h2 id="retention-settings"><span>02</span> Retention &amp; storage</h2>
        <label for="retention.preset">Retention preset</label>
        <select
          id="retention.preset"
          value={snapshot.values['retention.preset'].value}
          onchange={(event) =>
            save('retention.preset', event.currentTarget.value)}
        >
          <option value="minimal">Minimal</option><option value="balanced"
            >Balanced</option
          ><option value="long-term">Long-term</option><option value="advanced"
            >Advanced</option
          >
        </select>
        <small
          ><span>{snapshot.values['retention.preset'].source}</span><span
            >Applies live</span
          ></small
        >
        <details>
          <summary>Advanced tier durations</summary>
          {#each ['retention.raw', 'retention.one_minute', 'retention.fifteen_minute', 'retention.one_hour'] as key}
            <label for={key}
              >{key.replace('retention.', '').replaceAll('_', ' ')}</label
            >
            <input
              id={key}
              value={snapshot.values[key].value}
              onblur={(event) => save(key, event.currentTarget.value)}
            />
          {/each}
        </details>
        <label for="database.target_budget_bytes"
          >Database target budget (bytes)</label
        >
        <input
          id="database.target_budget_bytes"
          inputmode="numeric"
          value={snapshot.values['database.target_budget_bytes'].value}
          onblur={(event) =>
            save('database.target_budget_bytes', event.currentTarget.value)}
        />
        <p>
          At warning and critical levels Binnacle reports pressure and cleans
          expired data; it does not silently remove in-retention history.
        </p>
      </section>

      <section
        id="authentication"
        class="ledger-section"
        aria-labelledby="authentication-settings"
      >
        <h2 id="authentication-settings"><span>03</span> Authentication</h2>
        {#each ['sessions.idle_timeout', 'sessions.absolute_lifetime'] as key}
          <label for={key}
            >{key.endsWith('idle_timeout')
              ? 'Idle timeout'
              : 'Absolute lifetime'}</label
          >
          <input
            id={key}
            value={snapshot.values[key].value}
            onblur={(event) => save(key, event.currentTarget.value)}
          />
          <small
            >{snapshot.values[key].source} · Applies to session handling live</small
          >
        {/each}
      </section>

      <section
        id="appearance"
        class="ledger-section"
        aria-labelledby="appearance-settings"
      >
        <h2 id="appearance-settings"><span>04</span> Appearance</h2>
        <label for="settings-theme">Theme</label>
        <select
          id="settings-theme"
          bind:value={theme}
          onchange={() => appearance(theme, density)}
        >
          <option value="system">System</option><option value="dark"
            >Dark</option
          ><option value="light">Light</option>
        </select>
        <label for="settings-density">Density</label>
        <select
          id="settings-density"
          bind:value={density}
          onchange={() => appearance(theme, density)}
        >
          <option value="comfortable">Comfortable</option><option
            value="compact">Compact</option
          >
        </select>
      </section>

      <section
        id="privacy"
        class="ledger-section"
        aria-labelledby="privacy-settings"
      >
        <h2 id="privacy-settings"><span>05</span> Privacy &amp; network</h2>
        <p>Your metrics stay on this server. No product telemetry is sent.</p>
        <dl>
          {#each ['http.listen_address', 'docker.socket_path', 'paths.host_proc', 'paths.host_sys'] as key}
            <dt>{key}</dt>
            <dd>
              <code>{snapshot.values[key].value}</code><br /><small
                >{snapshot.values[key].source} · Restart/deployment change required</small
              >
            </dd>
          {/each}
        </dl>
      </section>

      <section
        id="system"
        class="ledger-section"
        aria-labelledby="system-settings"
      >
        <h2 id="system-settings"><span>06</span> System</h2>
        <p><a href="/settings/monitor-health">Monitor health</a></p>
        <p><a href="/settings/diagnostics">Diagnostics bundle</a></p>
        <p>
          Data directory: <code>{snapshot.values['paths.data_dir'].value}</code>
          · {snapshot.values['paths.data_dir'].source} · Restart required
        </p>
      </section>
      <section
        id="history"
        class="ledger-section danger-section"
        aria-labelledby="history-management"
      >
        <h2 id="history-management"><span>07</span> History management</h2>
        <HistoryDeletion />
      </section>
    </div>
  </div>
{/if}
