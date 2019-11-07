import { computed } from '@ember/object';
import { equal } from '@ember/object/computed';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { hasMany } from 'ember-data/relationships';
import { fragment, fragmentArray } from 'ember-data-model-fragments/attributes';
import shortUUIDProperty from '../utils/properties/short-uuid';
import ipParts from '../utils/ip-parts';

export default Model.extend({
  // Available from list response
  name: attr('string'),
  datacenter: attr('string'),
  nodeClass: attr('string'),
  isDraining: attr('boolean'),
  schedulingEligibility: attr('string'),
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
  drainStrategy: fragment('drain-strategy'),

  isEligible: equal('schedulingEligibility', 'eligible'),

  address: computed('httpAddr', function() {
    return ipParts(this.httpAddr).address;
  }),

  port: computed('httpAddr', function() {
    return ipParts(this.httpAddr).port;
  }),

  isPartial: computed('httpAddr', function() {
    return this.httpAddr == null;
  }),

  allocations: hasMany('allocations', { inverse: 'node' }),

  drivers: fragmentArray('node-driver'),
  events: fragmentArray('node-event'),

  detectedDrivers: computed('drivers.@each.detected', function() {
    return this.drivers.filterBy('detected');
  }),

  unhealthyDrivers: computed('detectedDrivers.@each.healthy', function() {
    return this.detectedDrivers.filterBy('healthy', false);
  }),

  unhealthyDriverNames: computed('unhealthyDrivers.@each.name', function() {
    return this.unhealthyDrivers.mapBy('name');
  }),

  // A status attribute that includes states not included in node status.
  // Useful for coloring and sorting nodes
  compositeStatus: computed('status', 'isEligible', function() {
    return this.isEligible ? this.status : 'ineligible';
  }),
});
