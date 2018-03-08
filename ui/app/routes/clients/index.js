import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchAll } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithWatchers, {
  setupController(controller) {
    controller.set('watcher', this.get('watch').perform());
    return this._super(...arguments);
  },

  watch: watchAll('node'),
  watchers: collect('watch'),
});
