import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  job: null,

  // Provide a value that is bound to a query param
  sortProperty: null,
  sortDescending: null,

  // Provide actions that require routing
  onNamespaceChange() {},
  gotoTaskGroup() {},
  gotoJob() {},

  breadcrumbs: computed('job.{name,id}', function() {
    const job = this.get('job');
    return [
      { label: 'Jobs', args: ['jobs'] },
      {
        label: job.get('name'),
        args: ['jobs.job', job],
      },
    ];
  }),
});
