import Component from '@ember/component';
import { computed } from '@ember/object';
import { inject as service } from '@ember/service';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';

export default Component.extend({
  system: service(),

  job: null,

  // Provide a value that is bound to a query param
  sortProperty: null,
  sortDescending: null,

  // Provide actions that require routing
  onNamespaceChange() {},
  gotoTaskGroup() {},
  gotoJob() {},

  // Set to a { title, description } to surface an error
  errorMessage: null,

  breadcrumbs: computed('job.{name,id}', function() {
    const job = this.get('job');
    return [
      { label: 'Jobs', args: ['jobs'] },
      {
        label: job.get('name'),
        args: [
          'jobs.job',
          job,
          qpBuilder({
            jobNamespace: job.get('namespace.name') || 'default',
          }),
        ],
      },
    ];
  }),

  actions: {
    clearErrorMessage() {
      this.set('errorMessage', null);
    },
    handleError(errorObject) {
      this.set('errorMessage', errorObject);
    },
  },
});
