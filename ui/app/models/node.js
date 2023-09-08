/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { computed } from '@ember/object';
import { equal } from '@ember/object/computed';
import Model from '@ember-data/model';
import { attr } from '@ember-data/model';
import { hasMany } from '@ember-data/model';
import { fragment, fragmentArray } from 'ember-data-model-fragments/attributes';
import RSVP from 'rsvp';
import shortUUIDProperty from '../utils/properties/short-uuid';
import ipParts from '../utils/ip-parts';
import classic from 'ember-classic-decorator';

@classic
export default class Node extends Model {
  // Available from list response
  @attr('string') name;
  @attr('string') datacenter;
  @attr('string') nodeClass;
  @attr('boolean') isDraining;
  @attr('string') schedulingEligibility;
  @attr('string') status;
  @attr('string') statusDescription;
  @shortUUIDProperty('id') shortId;
  @attr('number') modifyIndex;
  @attr('string') version;
  @attr('string') nodePool;

  // Available from single response
  @attr('string') httpAddr;
  @attr('boolean') tlsEnabled;
  @fragment('structured-attributes') attributes;
  @fragment('structured-attributes') meta;
  @fragment('resources') resources;
  @fragment('resources') reserved;
  @fragment('drain-strategy') drainStrategy;

  @equal('schedulingEligibility', 'eligible') isEligible;

  @computed('httpAddr')
  get address() {
    return ipParts(this.httpAddr).address;
  }

  @computed('httpAddr')
  get port() {
    return ipParts(this.httpAddr).port;
  }

  @computed('httpAddr')
  get isPartial() {
    return this.httpAddr == null;
  }

  @hasMany('allocations', { inverse: 'node' }) allocations;

  @computed('allocations.@each.clientStatus')
  get completeAllocations() {
    return this.allocations.filterBy('clientStatus', 'complete');
  }

  @computed('allocations.@each.isRunning')
  get runningAllocations() {
    return this.allocations.filterBy('isRunning');
  }

  @computed('allocations.@each.{isMigrating,isRunning}')
  get migratingAllocations() {
    return this.allocations.filter(
      (alloc) => alloc.isRunning && alloc.isMigrating
    );
  }

  @computed('allocations.@each.{isMigrating,isRunning,modifyTime}')
  get lastMigrateTime() {
    const allocation = this.allocations
      .filterBy('isRunning', false)
      .filterBy('isMigrating')
      .sortBy('modifyTime')
      .reverse()[0];
    if (allocation) {
      return allocation.modifyTime;
    }

    return undefined;
  }

  @fragmentArray('node-driver') drivers;
  @fragmentArray('node-event') events;
  @fragmentArray('host-volume') hostVolumes;

  @computed('drivers.@each.detected')
  get detectedDrivers() {
    return this.drivers.filterBy('detected');
  }

  @computed('detectedDrivers.@each.healthy')
  get unhealthyDrivers() {
    return this.detectedDrivers.filterBy('healthy', false);
  }

  @computed('unhealthyDrivers.@each.name')
  get unhealthyDriverNames() {
    return this.unhealthyDrivers.mapBy('name');
  }

  // A status attribute that includes states not included in node status.
  // Useful for coloring and sorting nodes
  @computed('isDraining', 'isEligible', 'status')
  get compositeStatus() {
    if (this.status === 'down') {
      return 'down';
    } else if (this.isDraining) {
      return 'draining';
    } else if (!this.isEligible) {
      return 'ineligible';
    } else {
      return this.status;
    }
  }

  @computed('isDraining', 'isEligible', 'status')
  get compositeStatusIcon() {
    if (this.isDraining || !this.isEligible) {
      return 'alert-circle-fill';
    } else if (this.status === 'down') {
      return 'cancel-circle-fill';
    } else if (this.status === 'initializing') {
      return 'node-init-circle-fill';
    }
    return 'check-circle-fill';
  }

  setEligible() {
    if (this.isEligible) return RSVP.resolve();
    // Optimistically update schedulingEligibility for immediate feedback
    this.set('schedulingEligibility', 'eligible');
    return this.store.adapterFor('node').setEligible(this);
  }

  setIneligible() {
    if (!this.isEligible) return RSVP.resolve();
    // Optimistically update schedulingEligibility for immediate feedback
    this.set('schedulingEligibility', 'ineligible');
    return this.store.adapterFor('node').setIneligible(this);
  }

  drain(drainSpec) {
    return this.store.adapterFor('node').drain(this, drainSpec);
  }

  forceDrain(drainSpec) {
    return this.store.adapterFor('node').forceDrain(this, drainSpec);
  }

  cancelDrain() {
    return this.store.adapterFor('node').cancelDrain(this);
  }

  async addMeta(newMeta) {
    let metaResponse = await this.store
      .adapterFor('node')
      .addMeta(this, newMeta);

    if (!this.meta) {
      this.set('meta', this.store.createFragment('structured-attributes'));
    }

    this.meta.recomputeRawProperties(metaResponse.Meta);
    return metaResponse;
  }
}
