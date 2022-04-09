import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { collect } from '@ember/object/computed';
import { watchRecord } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';
import notifyError from 'nomad-ui/utils/notify-error';
export default class AllocationRoute extends Route.extend(WithWatchers) {
  @service store;

  startWatchers(controller, model) {
    if (model) {
      controller.set('watcher', this.watch.perform(model));
    }
  }

  model() {
    // Preload the job for the allocation since it's required for the breadcrumb trail
    return super
      .model(...arguments)
      .then((allocation) =>
        allocation
          .get('job')
          .then(() => this.store.findAll('namespace')) // namespaces belong to a job and are an asynchronous relationship so we can peak them later on
          .then(() => allocation)
      )
      .catch(notifyError(this));
  }

  @watchRecord('allocation') watch;

  @collect('watch') watchers;
}
