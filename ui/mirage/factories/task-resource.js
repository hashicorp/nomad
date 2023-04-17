/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { Factory, trait } from 'ember-cli-mirage';
import { generateResources } from '../common';

export default Factory.extend({
  name: () => '!!!this should be set by the allocation that owns this task state!!!',

  resources: generateResources,
});
