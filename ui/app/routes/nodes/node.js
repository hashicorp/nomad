import Ember from 'ember';

const { Route, inject } = Ember;

export default Route.extend({
  store: inject.service(),

  afterModel(model) {
    if (model.get('isPartial')) {
      return model.reload().then(node => node.get('allocations'));
    }
    return model.get('allocations');
  },
});
