import Model from 'ember-data/model';
import attr from 'ember-data/attr';
// import { fragmentArray } from 'ember-data-model-fragments/attributes';

export default Model.extend({
  topologies: attr(),
  provider: attr('string'),
  version: attr('string'),
  controllerRequired: attr('boolean'),

  // controllers: fragmentArray('storage-controller', { defaultValue: () => [] }),
  // nodes: fragmentArray('storage-node', { defaultValue: () => [] }),
});
