/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { computed } from '@ember/object';
import Model from '@ember-data/model';
import { attr, belongsTo, hasMany } from '@ember-data/model';

export default class Volume extends Model {
  @attr('string') plainId;
  @attr('string') name;

  @belongsTo('namespace') namespace;
  @belongsTo('plugin') plugin;

  @hasMany('allocation') writeAllocations;
  @hasMany('allocation') readAllocations;

  @computed('writeAllocations.[]', 'readAllocations.[]')
  get allocations() {
    return [
      ...this.writeAllocations.toArray(),
      ...this.readAllocations.toArray(),
    ];
  }

  @attr('number') currentWriters;
  @attr('number') currentReaders;

  @computed('currentWriters', 'currentReaders')
  get allocationCount() {
    return this.currentWriters + this.currentReaders;
  }

  @attr('string') externalId;
  @attr() topologies;
  @attr('string') accessMode;
  @attr('string') attachmentMode;
  @attr('boolean') schedulable;
  @attr('string') provider;
  @attr('string') version;

  @attr('boolean') controllerRequired;
  @attr('number') controllersHealthy;
  @attr('number') controllersExpected;

  @computed('plainId')
  get idWithNamespace() {
    return `${this.plainId}@${this.belongsTo('namespace').id()}`;
  }

  @computed('controllersHealthy', 'controllersExpected')
  get controllersHealthyProportion() {
    return this.controllersHealthy / this.controllersExpected;
  }

  @attr('number') nodesHealthy;
  @attr('number') nodesExpected;

  @computed('nodesHealthy', 'nodesExpected')
  get nodesHealthyProportion() {
    return this.nodesHealthy / this.nodesExpected;
  }

  @attr('number') resourceExhausted;
  @attr('number') createIndex;
  @attr('number') modifyIndex;
}
