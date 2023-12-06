/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// import { readOnly } from '@ember/object/computed';
import Model from '@ember-data/model';
import { attr } from '@ember-data/model';

export default class Namespace extends Model {
  // TODO: Did I break something??
  @attr('string') name;
  @attr('string') hash;
  @attr('string') description;
}

// TODO: Nomitch - figure out what to do with the actual rules
