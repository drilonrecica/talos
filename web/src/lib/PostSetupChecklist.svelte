<script lang="ts">
  import { dismissChecklist, onboardingState } from './onboarding';
  let visible = $state(false);
  let docker = $state('Needs attention');
  let metadata = $state('Not detected');
  void onboardingState().then((state) => {
    visible = Boolean(state.completedAt && !state.checklistDismissed);
    docker =
      state.diagnostics?.find((item) => item.id === 'docker_api')?.status ===
      'passed'
        ? 'Done'
        : 'Needs attention';
    metadata =
      state.diagnostics?.find((item) => item.id === 'deployment_metadata')
        ?.status === 'passed'
        ? 'Done'
        : 'Not detected';
  });
  async function dismiss() {
    await dismissChecklist();
    visible = false;
  }
</script>

{#if visible}
  <aside class="card" aria-labelledby="checklist-title">
    <h2 id="checklist-title">Installation checklist</h2>
    <ul>
      <li>Create admin account — Done</li>
      <li>Host monitoring — Done</li>
      <li>Docker monitoring — {docker}</li>
      <li>Compose/Coolify resources — {metadata}</li>
      <li><a href="/alerts">Create a first health check</a> — Optional</li>
      <li><a href="/alerts">Review alert thresholds</a> — Recommended</li>
    </ul>
    <button type="button" onclick={dismiss}>Dismiss checklist</button>
  </aside>
{/if}
