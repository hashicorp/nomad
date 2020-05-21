import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchRecord } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithWatchers, {
  startWatchers(controller, model) {
    if (!model) return;

    controller.set('watchers', {
      model: this.watch.perform(model),
    });
  },

  watch: watchRecord('plugin'),
  watchers: collect('watch'),
});
