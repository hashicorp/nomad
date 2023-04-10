/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { Factory } from 'ember-cli-mirage';

export default Factory.extend({
  id: () => {
    throw new Error('The region factory will not generate IDs!');
  },
});
