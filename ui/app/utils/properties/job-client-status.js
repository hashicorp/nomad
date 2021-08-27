import { computed } from '@ember/object';

// An Ember.Computed property that persists set values in localStorage
// and will attempt to get its initial value from localStorage before
// falling back to a default.
//
// ex. showTutorial: localStorageProperty('nomadTutorial', true),
export default function jobClientStatus(nodesKey, jobStatusKey, jobAllocsKey) {
  return computed(nodesKey, jobStatusKey, jobAllocsKey, function() {
    const allocs = this.get(jobAllocsKey);
    const jobStatus = this.get(jobStatusKey);
    const nodes = this.get(nodesKey);

    return {
      byNode: {
        '123': 'running',
        '14b8ff31-8310-4877-b3e9-ba6ebcbf37a2': 'running',
        '193538ac-30d7-48eb-9a31-b89a83abae1d': 'degraded',
        '0cdce07c-6c50-4364-9ffd-e08c419f93aa': 'lost',
        'a07563e7-fdc6-4df4-bfd7-82b97a3605ab': 'failed',
        '6c32bb4d-6fda-4c0d-a7be-008f857d9f0e': 'queued',
        '75930e1c-15f2-4411-8dbd-1016e3d54c12': 'running',
        '8f834f8f-75cc-4da7-96f2-3665bd9bf774': 'running',
        '386a8b5f-68ab-4baf-a5ca-597d1bed8589': 'running',
      },
      byStatus: {
        running: ['123'],
      },
    };
  });
}
