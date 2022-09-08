import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { collect } from '@ember/object/computed';
import {
  watchRecord,
  watchNonStoreRecords,
} from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import notifyError from 'nomad-ui/utils/notify-error';
export default class AllocationRoute extends Route.extend(WithWatchers) {
  @service store;

  startWatchers(controller, model) {
    if (model) {
      controller.set('watcher', this.watch.perform(model));

      // Conditionally Long Poll /checks endpoint if alloc has nomad services
      const doesAllocHaveServices =
        !!model.taskGroup?.services?.filterBy('provider', 'nomad').length ||
        !!model.states
          ?.mapBy('task')
          ?.map((t) => t && t.get('services'))[0]
          ?.filterBy('provider', 'nomad').length;

      if (doesAllocHaveServices) {
        controller.set(
          'watchHealthChecks',
          this.watchHealthChecks.perform(model, 'getServiceHealth')
        );
      }
    }
  }

  model() {
    // Preload the job for the allocation since it's required for the breadcrumb trail
    return super
      .model(...arguments)
      .then((allocation) => {
        const jobId = allocation.belongsTo('job').id();
        return this.store
          .findRecord('job', jobId)
          .then(() => this.store.findAll('namespace')) // namespaces belong to a job and are an asynchronous relationship so we can peak them later on
          .then(() => allocation);
      })
      .catch(notifyError(this));
  }

  @watchRecord('allocation') watch;
  @watchNonStoreRecords('allocation') watchHealthChecks;

  @collect('watch', 'watchHealthChecks') watchers;
}
