/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { computed } from '@ember/object';
import Model from '@ember-data/model';
import { attr } from '@ember-data/model';
import { fragmentArray } from 'ember-data-model-fragments/attributes';

export default class Plugin extends Model {
  @attr('string') plainId;

  @attr() topologies;
  @attr('string') provider;
  @attr('string') version;

  @fragmentArray('storage-controller', { defaultValue: () => [] }) controllers;
  @fragmentArray('storage-node', { defaultValue: () => [] }) nodes;

  @attr('boolean') controllerRequired;
  @attr('number') controllersHealthy;
  @attr('number') controllersExpected;

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
}
