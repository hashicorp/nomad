import attr from 'ember-data/attr';
import { belongsTo } from 'ember-data/relationships';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  plugin: fragmentOwner(),

  node: belongsTo('node'),
  allocID: attr('string'),

  provider: attr('string'),
  version: attr('string'),
  healthy: attr('boolean'),
  healthDescription: attr('string'),
  updateTime: attr('date'),
  requiresControllerPlugin: attr('boolean'),
  requiresTopologies: attr('boolean'),

  nodeInfo: attr(),

  // Fragments can't have relationships, so provider a manual getter instead.
  async getAllocation() {
    return this.store.findRecord('allocation', this.allocID);
  },
});
