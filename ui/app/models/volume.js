import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo, hasMany } from 'ember-data/relationships';

export default Model.extend({
  name: attr('string'),

  namespace: belongsTo('namespace'),
  plugin: belongsTo('plugin'),
  allocations: hasMany('allocation'),

  externalID: attr('string'),
  topologies: attr(),
  accessMode: attr('string'),
  attachmentMode: attr('string'),
  schedulable: attr('boolean'),
  provider: attr('string'),
  version: attr('string'),

  controllerRequired: attr('boolean'),
  controllersHealthy: attr('number'),
  controllersExpected: attr('number'),

  nodesHealthy: attr('number'),
  nodesExpected: attr('number'),

  resourceExhausted: attr('number'),
  createIndex: attr('number'),
  modifyIndex: attr('number'),
});
