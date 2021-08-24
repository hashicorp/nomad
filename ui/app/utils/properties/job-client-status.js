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
      },
      byStatus: {
        running: ['123'],
      },
    };
  });
}
