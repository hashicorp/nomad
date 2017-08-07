import Ember from 'ember';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { hasMany } from 'ember-data/relationships';
import { fragment } from 'ember-data-model-fragments/attributes';
import shortUUIDProperty from '../utils/properties/short-uuid';

const { computed } = Ember;

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
  resources: fragment('resources'),
  reserved: fragment('resources'),

  address: computed('httpAddr', function() {
    const addr = this.get('httpAddr');
    return addr && addr.split(':')[0];
  }),

  port: computed('httpAddr', function() {
    const addr = this.get('httpAddr');
    return addr && addr.split(':')[1];
  }),

  isPartial: computed('httpAddr', function() {
    return this.get('httpAddr') == null;
  }),

  allocations: hasMany('allocations'),

  // Since allocations are fetched manually, tracking the status of fetching
  // the allocations must also be done manually
  allocationsIsLoaded: false,

  findAllocations() {
    const promise = this.store.adapterFor('node').findAllocations(this).then(() => {
      this.set('allocationsIsLoaded', true);
    });
    return promise;
  },
});
