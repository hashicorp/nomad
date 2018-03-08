import Route from '@ember/routing/route';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import { collect } from '@ember/object/computed';
import { watchRecord } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithModelErrorHandling, WithWatchers, {
  startWatchers(controller, model) {
    controller.set('watcher', this.get('watch').perform(model));
  },

  watch: watchRecord('allocation'),

  watchers: collect('watch'),
});
