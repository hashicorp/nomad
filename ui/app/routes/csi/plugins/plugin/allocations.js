import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchRecord } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default class AllocationsRoute extends Route.extend(WithWatchers) {
  startWatchers(controller, model) {
    if (!model) return;

    controller.set('watchers', {
      model: this.watch.perform(model),
    });
  }

  resetController(controller, isExiting) {
    if (isExiting) {
      controller.setProperties({
        currentPage: 1,
        qpType: '',
        qpHealth: '',
      });
    }
  }

  @watchRecord('plugin') watch;
  @collect('watch') watchers;
}
