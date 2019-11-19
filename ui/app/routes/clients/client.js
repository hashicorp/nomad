import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import notifyError from 'nomad-ui/utils/notify-error';
import { watchRecord, watchRelationship } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithWatchers, {
  store: service(),

  model() {
    return this._super(...arguments).catch(notifyError(this));
  },

  breadcrumbs(model) {
    if (!model) return [];
    return [
      {
        label: model.get('shortId'),
        args: ['clients.client', model.get('id')],
      },
    ];
  },

  afterModel(model) {
    if (model && model.get('isPartial')) {
      return model.reload().then(node => node.get('allocations'));
    }
    return model && model.get('allocations');
  },

  setupController(controller, model) {
    controller.set('flagAsDraining', model.isDraining);

    return this._super(...arguments);
  },

  resetController(controller) {
    controller.setProperties({
      eligibilityError: null,
      stopDrainError: null,
    });
  },

  startWatchers(controller, model) {
    if (model) {
      controller.set('watchModel', this.watch.perform(model));
      controller.set('watchAllocations', this.watchAllocations.perform(model));
    }
  },

  watch: watchRecord('node'),
  watchAllocations: watchRelationship('allocations'),

  watchers: collect('watch', 'watchAllocations'),
});
