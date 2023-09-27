/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check
import Model from '@ember-data/model';
import { attr, hasMany } from '@ember-data/model';

export default class Role extends Model {
  @attr('string') name;
  @attr('string') description;
  @hasMany('policy', { defaultValue: () => [] }) policies;
  @attr() policyNames;
}
