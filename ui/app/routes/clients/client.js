import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import notifyError from 'nomad-ui/utils/notify-error';
import { watchRecord, watchRelationship } from 'nomad-ui/utils/properties/watch';

export default Route.extend({
  store: service(),

  model() {
    return this._super(...arguments).catch(notifyError(this));
  },

  afterModel(model) {
    if (model && model.get('isPartial')) {
      return model.reload().then(node => node.get('allocations'));
    }
    return model && model.get('allocations');
  },

  setupController(controller, model) {
    controller.set('watchModel', this.get('watch').perform(model));
    controller.set('watchAllocations', this.get('watchAllocations').perform(model));
    return this._super(...arguments);
  },

  deactivate() {
    this.get('watch').cancelAll();
    this.get('watchAllocations').cancelAll();
    return this._super(...arguments);
  },

  watch: watchRecord('node'),
  watchAllocations: watchRelationship('allocations'),
});
