import { computed } from '@ember/object';
import attr from 'ember-data/attr';
import { belongsTo } from 'ember-data/relationships';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';
import PromiseObject from 'nomad-ui/utils/classes/promise-object';

export default Fragment.extend({
  plugin: fragmentOwner(),

  node: belongsTo('node'),
  allocID: attr('string'),

  // Model fragments don't support relationships, but with an allocation ID
  // a "belongsTo" can be sufficiently mocked.
  allocation: computed('allocID', function() {
    if (!this.allocID) return null;
    return PromiseObject.create({
      promise: this.store.findRecord('allocation', this.allocID),
      reload: () => this.store.findRecord('allocation', this.allocID),
    });
  }),

  provider: attr('string'),
  version: attr('string'),
  healthy: attr('boolean'),
  healthDescription: attr('string'),
  updateTime: attr('date'),
  requiresControllerPlugin: attr('boolean'),
  requiresTopologies: attr('boolean'),

  nodeInfo: attr(),
});
