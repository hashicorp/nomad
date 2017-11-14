import Ember from 'ember';
import notifyError from 'nomad-ui/utils/notify-error';

const { Route, inject } = Ember;

export default Route.extend({
  store: inject.service(),

  model() {
    return this._super(...arguments).catch(notifyError(this));
  },

  afterModel(model) {
    if (model && model.get('isPartial')) {
      return model.reload().then(node => node.get('allocations'));
    }
    return model && model.get('allocations');
  },
});
