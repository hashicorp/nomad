import { computed } from '@ember/object';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { hasMany } from 'ember-data/relationships';
import { fragment } from 'ember-data-model-fragments/attributes';
import shortUUIDProperty from '../utils/properties/short-uuid';
import ipParts from '../utils/ip-parts';

export default Model.extend({
  // Available from list response
  name: attr('string'),
  datacenter: attr('string'),
  isDraining: attr('boolean'),
  status: attr('string'),
  statusDescription: attr('string'),
  shortId: shortUUIDProperty('id'),
  modifyIndex: attr('number'),

  // Available from single response
  httpAddr: attr('string'),
  tlsEnabled: attr('boolean'),
  attributes: fragment('node-attributes'),
  meta: fragment('node-attributes'),
  resources: fragment('resources'),
  reserved: fragment('resources'),

  address: computed('httpAddr', function() {
    return ipParts(this.get('httpAddr')).address;
  }),

  port: computed('httpAddr', function() {
    return ipParts(this.get('httpAddr')).port;
  }),

  isPartial: computed('httpAddr', function() {
    return this.get('httpAddr') == null;
  }),

  allocations: hasMany('allocations', { inverse: 'node' }),
});
