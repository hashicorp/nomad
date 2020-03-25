import attr from 'ember-data/attr';
import { belongsTo } from 'ember-data/relationships';
import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default Fragment.extend({
  plugin: fragmentOwner(),

  node: belongsTo('node'),
  allocation: belongsTo('allocation'),

  provider: attr('string'),
  version: attr('string'),
  healthy: attr('boolean'),
  healthDescription: attr('string'),
  updateTime: attr('date'),
  requiresControllerPlugin: attr('boolean'),
  requiresTopologies: attr('boolean'),

  nodeInfo: attr(),
});
