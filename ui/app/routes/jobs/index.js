import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchAll } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithWatchers, {
  setupController(controller) {
    controller.set('modelWatch', this.get('watch').perform());
    return this._super(...arguments);
  },

  watch: watchAll('job'),
  watchers: collect('watch'),

  actions: {
    refreshRoute() {
      return true;
    },
  },
});
