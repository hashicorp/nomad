import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import EmberError from '@ember/error';
import { resolve } from 'rsvp';
import { watchRecord, watchRelationship } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';
import notifyError from 'nomad-ui/utils/notify-error';

export default Route.extend(WithWatchers, {
  breadcrumbs(model) {
    if (!model) return [];
    return [
      {
        label: model.get('name'),
        args: [
          'jobs.job.task-group',
          model.get('job'),
          model.get('name'),
          qpBuilder({ jobNamespace: model.get('job.namespace.name') || 'default' }),
        ],
      },
    ];
  },

  model({ name }) {
    const job = this.modelFor('jobs.job');

    // If there is no job, then there is no task group.
    // Let the job route handle the 404.
    if (!job) return;

    // If the job is a partial (from the list request) it won't have task
    // groups. Reload the job to ensure task groups are present.
    const reload = job.get('isPartial') ? job.reload() : resolve();
    return reload
      .then(() => {
        const taskGroup = job.get('taskGroups').findBy('name', name);
        if (!taskGroup) {
          const err = new EmberError(`Task group ${name} for job ${job.get('name')} not found`);
          err.code = '404';
          throw err;
        }

        // Refresh job allocations before-hand (so page sort works on load)
        return job
          .hasMany('allocations')
          .reload()
          .then(() => taskGroup);
      })
      .catch(notifyError(this));
  },

  startWatchers(controller, model) {
    if (model) {
      const job = model.get('job');
      controller.set('watchers', {
        job: this.get('watchJob').perform(job),
        summary: this.get('watchSummary').perform(job.get('summary')),
        allocations: this.get('watchAllocations').perform(job),
      });
    }
  },

  watchJob: watchRecord('job'),
  watchSummary: watchRecord('job-summary'),
  watchAllocations: watchRelationship('allocations'),

  watchers: collect('watchJob', 'watchSummary', 'watchAllocations'),
});
