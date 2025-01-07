/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import Model from '@ember-data/model';
import { attr } from '@ember-data/model';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import formatHost from 'nomad-ui/utils/format-host';

export default class Agent extends Model {
  @service system;

  @attr('string') name;
  @attr('string') address;
  @attr('string') serfPort;
  @attr('string') rpcPort;
  @attr({ defaultValue: () => ({}) }) tags;
  @attr('string') status;
  @attr('string') datacenter;
  @attr('string') region;

  @computed('address', 'port')
  get rpcAddr() {
    const { address, rpcPort } = this;
    return formatHost(address, rpcPort);
  }

  @tracked isLeader = false;

  @action async checkForLeadership() {
    const leaders = await this.system.leaders;
    this.isLeader = leaders.includes(this.rpcAddr);
    return this.isLeader;
  }

  @computed('tags.build')
  get version() {
    return this.tags?.build || '';
  }
}
