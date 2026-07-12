<script lang="ts">
  import {
    completeOnboarding,
    onboardingState,
    runDiagnostics,
    saveOnboarding,
    type OnboardingState,
  } from './onboarding';
  import Badge from './ui/Badge.svelte';
  let { oncomplete }: { oncomplete: () => void } = $props();
  let onboarding = $state<OnboardingState>({ checklistDismissed: false });
  let exposure = $state('private');
  let retention = $state('balanced');
  let outbound = $state(false);
  let busy = $state(false);
  let error = $state('');

  void onboardingState()
    .then((value) => {
      onboarding = value;
      exposure = value.exposureMode ?? 'private';
      retention = value.retentionPreset ?? 'balanced';
    })
    .catch((reason) => (error = String(reason)));

  async function diagnose() {
    busy = true;
    error = '';
    try {
      onboarding = (await saveOnboarding(exposure, retention)) ?? onboarding;
      onboarding = (await runDiagnostics(outbound)) ?? onboarding;
    } catch (reason) {
      error = reason instanceof Error ? reason.message : 'Diagnostics failed.';
    } finally {
      busy = false;
    }
  }
  async function finish() {
    busy = true;
    error = '';
    try {
      await completeOnboarding();
      oncomplete();
    } catch (reason) {
      error = reason instanceof Error ? reason.message : 'Onboarding failed.';
    } finally {
      busy = false;
    }
  }
</script>

<section class="onboarding" aria-labelledby="onboarding-title">
  <header class="commission-header">
    <span>COMMISSIONING / SEQUENCE</span>
    <h1 id="onboarding-title">Finish installation</h1>
    <p>
      Your metrics stay on this server. Binnacle sends no product telemetry.
    </p>
  </header>
  {#if error}<p role="alert">{error}</p>{/if}
  <section class="commission-step">
    <span class="step-number">01</span>
    <div>
      <fieldset>
        <legend>Access exposure</legend>
        <label
          ><input type="radio" bind:group={exposure} value="private" /> Private access</label
        >
        <label
          ><input type="radio" bind:group={exposure} value="public" /> Public HTTPS
          URL</label
        >
        <p>
          Public access should use HTTPS and an additional access control where
          practical.
        </p>
      </fieldset>
    </div>
  </section>
  <section class="commission-step">
    <span class="step-number">02</span>
    <div>
      <h2>Retention</h2>
      <label for="retention">Retention preset</label>
      <select id="retention" bind:value={retention}>
        <option value="minimal">Minimal</option>
        <option value="balanced">Balanced</option>
        <option value="long-term">Long-term</option>
      </select>
    </div>
  </section>
  <section class="commission-step">
    <span class="step-number">03</span>
    <div>
      <h2>Diagnostics</h2>
      <label
        ><input type="checkbox" bind:checked={outbound} /> Test outbound HTTPS (optional)</label
      >
      <button type="button" disabled={busy} onclick={diagnose}
        >Run installation diagnostics</button
      >
    </div>
  </section>
  {#if onboarding.diagnostics?.length}
    <section class="commission-step" aria-labelledby="diagnostics-title">
      <span class="step-number">04</span>
      <div>
        <h2 id="diagnostics-title">Diagnostics</h2>
        {#each onboarding.diagnostics as check (check.id)}
          <article class="diagnostic-result">
            <h3>{check.name}</h3>
            <Badge
              state={check.status === 'passed'
                ? 'healthy'
                : check.status === 'failed'
                  ? 'down'
                  : 'unknown'}>{check.status}</Badge
            >
            <p>{check.reason}</p>
            {#if check.suggestedFix}<p>{check.suggestedFix}</p>{/if}
            {#if check.technicalDetail}<details>
                <summary>Technical detail</summary><code
                  >{check.technicalDetail}</code
                >
              </details>{/if}
          </article>
        {/each}
        <button type="button" disabled={busy} onclick={finish}
          >Enter dashboard</button
        >
      </div>
    </section>
  {/if}
</section>
