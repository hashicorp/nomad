/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import { attr, belongsTo } from '@ember-data/model';
import Model from '@ember-data/model';
import { alias } from '@ember/object/computed';

export default class Service extends Model {
  @belongsTo('allocation') allocation;
  @belongsTo('job') job;
  @belongsTo('node') node;

  @attr('string') address;
  @attr('number') createIndex;
  @attr('string') datacenter;
  @attr('number') modifyIndex;
  @attr('string') namespace;
  @attr('number') port;
  @attr('string') serviceName;
  @attr() tags;
  @attr() canary_tags;

  @alias('serviceName') name;

  // Services can exist at either Group or Task level.
  // While our endpoints to get them do not explicitly tell us this,
  // we can infer it from the service's ID:
  get derivedLevel() {
    const idWithoutServiceName = this.id.replace(this.serviceName, '');
    if (idWithoutServiceName.includes('group-')) {
      return 'group';
    } else {
      return 'task';
    }
  }
}
