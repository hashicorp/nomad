import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchRecord } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import notifyError from 'nomad-ui/utils/notify-error';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';
import { jobCrumbs } from 'nomad-ui/utils/breadcrumb-utils';

export default Route.extend(WithWatchers, {
  startWatchers(controller, model) {
    if (model) {
      controller.set('watcher', this.get('watch').perform(model));
    }
  },

  // Allocation breadcrumbs extend from job / task group breadcrumbs
  // even though the route structure does not.
  breadcrumbs(model) {
    const jobQueryParams = qpBuilder({
      jobNamespace: model.get('job.namespace.name') || 'default',
    });

    return [
      { label: 'Jobs', args: ['jobs.index', jobQueryParams] },
      ...jobCrumbs(model.get('job')),
      {
        label: model.get('taskGroupName'),
        args: [
          'jobs.job.task-group',
          model.get('job.plainId'),
          model.get('taskGroupName'),
          jobQueryParams,
        ],
      },
      {
        label: model.get('shortId'),
        args: ['allocations.allocation', model],
      },
    ];
  },

  model() {
    // Preload the job for the allocation since it's required for the breadcrumb trail
    return this._super(...arguments)
      .then(allocation => allocation.get('job').then(() => allocation))
      .catch(notifyError(this));
  },

  watch: watchRecord('allocation'),

  watchers: collect('watch'),
});
