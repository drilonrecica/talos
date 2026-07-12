<script lang="ts">
  import { onMount } from 'svelte';
  import { LiveStore } from './lib/live.svelte';
  import {
    applyPreferences,
    preferences,
    type Density,
    type Theme,
  } from './lib/preferences';
  import { t } from './lib/i18n';
  import Overview from './lib/Overview.svelte';
  import Server from './lib/Server.svelte';
  import Resources from './lib/Resources.svelte';
  import Events from './lib/Events.svelte';
  import ResourceDetail from './lib/ResourceDetail.svelte';
  import Settings from './lib/Settings.svelte';
  import Login from './lib/Login.svelte';
  import SessionControls from './lib/SessionControls.svelte';
  import { currentSession, type SessionInfo } from './lib/auth';
  import { onboardingState, setupAvailable } from './lib/onboarding';
  import Setup from './lib/Setup.svelte';
  import Onboarding from './lib/Onboarding.svelte';
  import Diagnostics from './lib/Diagnostics.svelte';
  import MonitorHealth from './lib/MonitorHealth.svelte';
  import ConnectionNotice from './lib/ConnectionNotice.svelte';

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
  let resourceID = $state(location.pathname.split('/')[2] || '');
  let theme = $state<Theme>('system');
  let density = $state<Density>('comfortable');
  let session = $state<SessionInfo | null>(null);
  onMount(() => {
    ({ theme, density } = preferences());
    applyPreferences({ theme, density });
    void currentSession()
      .catch(() => null)
      .then((value) => {
        session = value;
        allowed = value !== null;
        loading = false;
        if (allowed) {
          void onboardingState().then((onboarding) => {
            if (!onboarding.completedAt && route !== 'onboarding') {
              history.replaceState({}, '', '/onboarding');
              route = 'onboarding';
            }
          });
          live.connect();
        } else if (route !== 'login' && route !== 'setup') {
          void setupAvailable().then((available) => {
            history.replaceState({}, '', available ? '/setup' : '/login');
            route = available ? 'setup' : 'login';
          });
        }
      });
    return () => live.close();
  });
  function setTheme(value: Theme) {
    theme = value;
    applyPreferences({ theme, density });
  }
  function setDensity(value: Density) {
    density = value;
    applyPreferences({ theme, density });
  }
  function authenticated(path: string) {
    void currentSession().then((value) => {
      if (!value) return;
      session = value;
      allowed = true;
      history.replaceState({}, '', path);
      route = path.split('/')[1] || 'overview';
      resourceID = path.split('/')[2] || '';
      live.connect();
    });
  }
  function signedOut() {
    live.close();
    session = null;
    allowed = false;
    route = 'login';
    history.replaceState({}, '', '/login');
  }
  function setupClaimed() {
    authenticated('/onboarding');
  }
  function onboardingComplete() {
    history.replaceState({}, '', '/overview');
    route = 'overview';
  }
</script>

<svelte:head><title>Binnacle</title></svelte:head>
<a class="skip" href="#content">{t('shell.skip')}</a>
{#if loading}
  <main aria-busy="true"><p>{t('shell.access')}</p></main>
{:else if !allowed}
  <main id="content">
    {#if route === 'setup'}<Setup onclaimed={setupClaimed} />
    {:else}<Login onauthenticated={authenticated} />{/if}
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
        }}>Binnacle</a
      ><span>{t('shell.live')}</span>
      <label
        >Theme <select
          value={theme}
          onchange={(e) => setTheme(e.currentTarget.value as Theme)}
          ><option value="system">System</option><option value="dark"
            >Dark</option
          ><option value="light">Light</option></select
        ></label
      >
      {#if session}<SessionControls {session} onlogout={signedOut} />{/if}
      <label
        >Density <select
          value={density}
          onchange={(e) => setDensity(e.currentTarget.value as Density)}
          ><option value="comfortable">Comfortable</option><option
            value="compact">Compact</option
          ></select
        ></label
      >
    </header>
    <nav aria-label="Primary navigation">
      {#each routes as item (item)}<a
          href="/{item}"
          aria-current={route === item ? 'page' : undefined}
          onclick={(e) => {
            e.preventDefault();
            history.pushState({}, '', `/${item}`);
            route = item;
            resourceID = '';
          }}>{item}</a
        >{/each}
    </nav>
    <main id="content">
      <ConnectionNotice {live} />
      {#if route !== 'onboarding'}<h1>
          {route[0].toUpperCase() + route.slice(1)}
        </h1>{/if}
      {#if route === 'onboarding'}<Onboarding oncomplete={onboardingComplete} />
      {:else if route === 'overview'}<Overview
          {live}
        />{:else if route === 'resources' && resourceID}<ResourceDetail
          {live}
          id={resourceID}
        />{:else if route === 'resources'}<Resources
          {live}
        />{:else if route === 'server'}<Server
          {live}
        />{:else if route === 'events'}<Events
          {live}
        />{:else if route === 'checks'}<p>
          Checks are planned for a later release.
        </p>{:else if route === 'settings' && resourceID === 'monitor-health'}<MonitorHealth
        />{:else if route === 'settings' && resourceID === 'diagnostics'}<Diagnostics
        />{:else if route === 'settings'}<Settings />{:else}<p>
          {live.state === 'connected'
            ? 'Live connection active.'
            : 'Connecting to live monitoring…'}
        </p>{/if}
    </main>
  </div>
{/if}
