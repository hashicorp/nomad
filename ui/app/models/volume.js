import { computed } from '@ember/object';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo, hasMany } from 'ember-data/relationships';

export default Model.extend({
  plainId: attr('string'),
  name: attr('string'),

  namespace: belongsTo('namespace'),
  plugin: belongsTo('plugin'),

  writeAllocations: hasMany('allocation'),
  readAllocations: hasMany('allocation'),

  allocations: computed('writeAllocations.[]', 'readAllocations.[]', function() {
    return [...this.writeAllocations.toArray(), ...this.readAllocations.toArray()];
  }),

  externalId: attr('string'),
  topologies: attr(),
  accessMode: attr('string'),
  attachmentMode: attr('string'),
  schedulable: attr('boolean'),
  provider: attr('string'),
  version: attr('string'),

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

  resourceExhausted: attr('number'),
  createIndex: attr('number'),
  modifyIndex: attr('number'),
});
