<script lang="ts">
  import { onMount } from 'svelte';
  import { LiveStore } from './lib/live.svelte';
  import { applyPreferences, preferences } from './lib/preferences';
  import { t } from './lib/i18n';
  import Watch from './lib/Watch.svelte';
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
  const navigation = [
    { route: 'watch', label: 'Watch' },
    { route: 'resources', label: 'Resources' },
    { route: 'server', label: 'Server' },
    { route: 'events', label: 'Events' },
    { route: 'settings', label: 'Settings' },
  ] as const;
  const protectedRoutes = new Set([
    ...navigation.map((item) => item.route),
    'onboarding',
  ]);
  const publicRoutes = new Set(['login', 'setup']);

  let loading = $state(true);
  let allowed = $state(false);
  let route = $state('watch');
  let resourceID = $state('');
  let inspectID = $state('');
  let session = $state<SessionInfo | null>(null);

  function readLocation() {
    const parts = location.pathname.split('/').filter(Boolean);
    route = parts[0] || 'watch';
    resourceID = parts[1] || '';
    inspectID =
      route === 'watch'
        ? new URLSearchParams(location.search).get('inspect')?.trim() || ''
        : '';
  }

  function navigate(path: string, replace = false) {
    if (replace) history.replaceState({}, '', path);
    else history.pushState({}, '', path);
    readLocation();
  }

  function navigateFromLink(event: MouseEvent, path: string) {
    if (
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    )
      return;
    event.preventDefault();
    navigate(path);
  }

  function inspect(resource: string | null) {
    navigate(
      resource ? `/watch?inspect=${encodeURIComponent(resource)}` : '/watch',
    );
  }

  readLocation();
  onMount(() => {
    const preference = preferences();
    applyPreferences(preference);
    const onPopState = () => readLocation();
    addEventListener('popstate', onPopState);
    void currentSession()
      .catch(() => null)
      .then((value) => {
        session = value;
        allowed = value !== null;
        loading = false;
        if (allowed) {
          if (
            !protectedRoutes.has(route as (typeof navigation)[number]['route'])
          ) {
            navigate('/watch', true);
          }
          void onboardingState().then((onboarding) => {
            if (!onboarding.completedAt && route !== 'onboarding') {
              navigate('/onboarding', true);
            }
          });
          live.connect();
        } else if (!publicRoutes.has(route)) {
          void setupAvailable().then((available) => {
            navigate(available ? '/setup' : '/login', true);
          });
        }
      });
    return () => {
      removeEventListener('popstate', onPopState);
      live.close();
    };
  });

  function authenticated(path: string) {
    void currentSession().then((value) => {
      if (!value) return;
      session = value;
      allowed = true;
      navigate(path, true);
      live.connect();
    });
  }

  function signedOut() {
    live.close();
    session = null;
    allowed = false;
    navigate('/login', true);
  }

  function setupClaimed() {
    authenticated('/onboarding');
  }

  function onboardingComplete() {
    navigate('/watch', true);
  }
</script>

<svelte:head
  ><title>Binnacle — {route === 'watch' ? 'Watch' : route}</title></svelte:head
>
<a class="skip" href="#content">{t('shell.skip')}</a>
{#if loading}
  <main class="access-state" aria-busy="true">
    <img src="/brand/binnacle-mark-dark.png" alt="" />
    <p>{t('shell.access')}</p>
  </main>
{:else if !allowed}
  <main id="content" class="public-shell">
    {#if route === 'setup'}<Setup onclaimed={setupClaimed} />
    {:else}<Login onauthenticated={authenticated} />{/if}
  </main>
{:else if route === 'onboarding'}
  <main id="content" class="public-shell onboarding-shell">
    <Onboarding oncomplete={onboardingComplete} />
  </main>
{:else}
  <div class="console-shell">
    <header class="console-header">
      <a
        class="app-brand"
        href="/watch"
        onclick={(event) => navigateFromLink(event, '/watch')}
      >
        <img
          class="brand-logo-dark"
          src="/brand/binnacle-mark-dark.png"
          alt=""
        />
        <img class="brand-logo-light" src="/brand/binnacle-mark.png" alt="" />
        <span>Binnacle</span>
      </a>
      <div class="console-state" data-state={live.state}>
        <span class="state-signal" aria-hidden="true"></span>
        <strong
          >{live.state === 'connected'
            ? 'LIVE'
            : live.state.toUpperCase()}</strong
        >
        {#if live.snapshot}
          <span class="snapshot-time"
            >sample <time datetime={live.snapshot.ts}
              >{new Date(live.snapshot.ts).toLocaleTimeString()}</time
            ></span
          >
        {/if}
      </div>
      {#if session}<SessionControls {session} onlogout={signedOut} />{/if}
    </header>

    <main id="content" class:watch-main={route === 'watch'}>
      <ConnectionNotice {live} />
      {#if route === 'watch'}<Watch {live} {inspectID} oninspect={inspect} />
      {:else if route === 'resources' && resourceID}<ResourceDetail
          {live}
          id={resourceID}
        />
      {:else if route === 'resources'}<Resources {live} />
      {:else if route === 'server'}<Server {live} />
      {:else if route === 'events'}<Events {live} />
      {:else if route === 'settings' && resourceID === 'monitor-health'}<MonitorHealth
        />
      {:else if route === 'settings' && resourceID === 'diagnostics'}<Diagnostics
        />
      {:else if route === 'settings'}<Settings />{/if}
    </main>

    <nav class="command-deck" aria-label="Primary navigation">
      {#each navigation as item, index (item.route)}
        <a
          href={`/${item.route}`}
          aria-current={route === item.route ? 'page' : undefined}
          onclick={(event) => navigateFromLink(event, `/${item.route}`)}
          ><span class="command-number" aria-hidden="true">0{index + 1}</span
          >{item.label}</a
        >
      {/each}
    </nav>
  </div>
{/if}
