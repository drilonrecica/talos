<script lang="ts">
  import { onMount } from 'svelte';
  import { LiveStore, sessionActive } from './lib/live';
  import { applyPreferences, preferences, type Density, type Theme } from './lib/preferences';
  import { t } from './lib/i18n';
  import Overview from './lib/Overview.svelte';
  import Server from './lib/Server.svelte';
  import Resources from './lib/Resources.svelte';

  const live = new LiveStore();
  const routes = [
    'overview',
    'resources',
    'server',
    'events',
    'checks',
    'settings',
  ];
  let loading = $state(true);
  let allowed = $state(false);
  let route = $state(location.pathname.split('/')[1] || 'overview');
  let theme = $state<Theme>('system');
  let density = $state<Density>('comfortable');
  onMount(() => {
    ({ theme, density } = preferences());
    applyPreferences({ theme, density });
    void sessionActive()
      .catch(() => false)
      .then((active) => {
        allowed = active;
        loading = false;
        if (allowed) live.connect();
        else if (route !== 'login' && route !== 'setup') {
          history.pushState({}, '', '/login');
          route = 'login';
        }
      });
    return () => live.close();
  });
  function setTheme(value: Theme) { theme = value; applyPreferences({ theme, density }); }
  function setDensity(value: Density) { density = value; applyPreferences({ theme, density }); }
</script>

<svelte:head><title>TALOS</title></svelte:head>
<a class="skip" href="#content">{t('shell.skip')}</a>
{#if loading}
  <main aria-busy="true"><p>{t('shell.access')}</p></main>
{:else if !allowed}
  <main id="content">
    <h1>{route === 'setup' ? 'Setup TALOS' : 'Sign in to TALOS'}</h1>
    <p>Authentication is not configured in this build.</p>
  </main>
{:else}
  <div class="shell">
    <header>
      <a
        href="/overview"
        onclick={(e) => {
          e.preventDefault();
          history.pushState({}, '', '/overview');
          route = 'overview';
        }}>TALOS</a
      ><span>{t('shell.live')}</span>
      <label>Theme <select value={theme} onchange={(e) => setTheme(e.currentTarget.value as Theme)}><option value="system">System</option><option value="dark">Dark</option><option value="light">Light</option></select></label>
      <label>Density <select value={density} onchange={(e) => setDensity(e.currentTarget.value as Density)}><option value="comfortable">Comfortable</option><option value="compact">Compact</option></select></label>
    </header>
    <nav aria-label="Primary navigation">
      {#each routes as item (item)}<a
          href="/{item}"
          aria-current={route === item ? 'page' : undefined}
          onclick={(e) => {
            e.preventDefault();
            history.pushState({}, '', `/${item}`);
            route = item;
          }}>{item}</a
        >{/each}
    </nav>
    <main id="content">
      <h1>{route[0].toUpperCase() + route.slice(1)}</h1>
      {#if route === 'overview'}<Overview {live} />{:else if route === 'resources'}<Resources {live} />{:else if route === 'server'}<Server {live} />{:else if route === 'checks'}<p>
          Checks are planned for a later release.
        </p>{:else}<p>
          {live.state === 'connected'
            ? 'Live connection active.'
            : 'Connecting to live monitoring…'}
        </p>{/if}
    </main>
  </div>
{/if}
