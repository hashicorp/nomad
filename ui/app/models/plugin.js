import { computed } from '@ember/object';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { fragmentArray } from 'ember-data-model-fragments/attributes';

export default Model.extend({
  plainId: attr('string'),

  topologies: attr(),
  provider: attr('string'),
  version: attr('string'),

  controllers: fragmentArray('storage-controller', { defaultValue: () => [] }),
  nodes: fragmentArray('storage-node', { defaultValue: () => [] }),

  controllerRequired: attr('boolean'),
  controllersHealthy: attr('number'),
  controllersExpected: attr('number'),

  controllersHealthyProportion: computed('controllersHealthy', 'controllersExpected', function() {
    return this.controllersHealthy / this.controllersExpected;
  }),

  nodesHealthy: attr('number'),
  nodesExpected: attr('number'),

  nodesHealthyProportion: computed('nodesHealthy', 'nodesExpected', function() {
    return this.nodesHealthy / this.nodesExpected;
  }),
});
