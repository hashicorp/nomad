/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model from '@ember-data/model';
import { attr } from '@ember-data/model';
import { fragment } from 'ember-data-model-fragments/attributes';

export default class Namespace extends Model {
  @attr('string') name;
  @attr('string') hash;
  @attr('string') description;
  @attr('string') quota;
  @attr() meta;
  @fragment('ns-capabilities') capabilities;
  @fragment('ns-node-pool-configuration') nodePoolConfiguration;
}
