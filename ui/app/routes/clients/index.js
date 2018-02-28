import Route from '@ember/routing/route';
import { watchAll } from 'nomad-ui/utils/properties/watch';

export default Route.extend({
  setupController(controller) {
    controller.set('watcher', this.get('watch').perform());
    return this._super(...arguments);
  },

  deactivate() {
    this.get('watch').cancelAll();
    this._super(...arguments);
  },

  watch: watchAll('node'),
});
