/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// import { readOnly } from '@ember/object/computed';
import Model from '@ember-data/model';
import { attr } from '@ember-data/model';

export default class Namespace extends Model {
  @attr('string') name;
  @attr('string') hash;
  @attr('string') description;
}
