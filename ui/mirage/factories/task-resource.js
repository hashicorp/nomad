/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory, trait } from 'miragejs';
import { generateResources } from '../common';

export default Factory.extend({
  name: () => '!!!this should be set by the allocation that owns this task state!!!',

  resources: generateResources,
});
