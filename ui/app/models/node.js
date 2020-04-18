import { computed } from '@ember/object';
import { equal } from '@ember/object/computed';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { hasMany } from 'ember-data/relationships';
import { fragment, fragmentArray } from 'ember-data-model-fragments/attributes';
import RSVP from 'rsvp';
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
  completeAllocations: computed('allocations.@each.clientStatus', function() {
    return this.allocations.filterBy('clientStatus', 'complete');
  }),
  runningAllocations: computed('allocations.@each.isRunning', function() {
    return this.allocations.filterBy('isRunning');
  }),
  migratingAllocations: computed('allocations.@each.{isMigrating,isRunning}', function() {
    return this.allocations.filter(alloc => alloc.isRunning && alloc.isMigrating);
  }),
  lastMigrateTime: computed('allocations.@each.{isMigrating,isRunning,modifyTime}', function() {
    const allocation = this.allocations
      .filterBy('isRunning', false)
      .filterBy('isMigrating')
      .sortBy('modifyTime')
      .reverse()[0];
    if (allocation) {
      return allocation.modifyTime;
    }
  }),

  drivers: fragmentArray('node-driver'),
  events: fragmentArray('node-event'),
  hostVolumes: fragmentArray('host-volume'),

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
  compositeStatus: computed('isDraining', 'isEligible', 'status', function() {
    if (this.isDraining) {
      return 'draining';
    } else if (!this.isEligible) {
      return 'ineligible';
    } else {
      return this.status;
    }
  }),

  compositeStatusIcon: computed('isDraining', 'isEligible', 'status', function() {
    if (this.isDraining || !this.isEligible) {
      return 'alert-circle-fill';
    } else if (this.status === 'down') {
      return 'cancel-circle-fill';
    } else if (this.status === 'initializing') {
      return 'node-init-circle-fill';
    }
    return 'check-circle-fill';
  }),

  setEligible() {
    if (this.isEligible) return RSVP.resolve();
    // Optimistically update schedulingEligibility for immediate feedback
    this.set('schedulingEligibility', 'eligible');
    return this.store.adapterFor('node').setEligible(this);
  },

  setIneligible() {
    if (!this.isEligible) return RSVP.resolve();
    // Optimistically update schedulingEligibility for immediate feedback
    this.set('schedulingEligibility', 'ineligible');
    return this.store.adapterFor('node').setIneligible(this);
  },

  drain(drainSpec) {
    return this.store.adapterFor('node').drain(this, drainSpec);
  },

  forceDrain(drainSpec) {
    return this.store.adapterFor('node').forceDrain(this, drainSpec);
  },

  cancelDrain() {
    return this.store.adapterFor('node').cancelDrain(this);
  },
});
