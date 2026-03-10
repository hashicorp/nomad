/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { Factory } from 'ember-cli-mirage';

export default Factory.extend({
  id: () => {
    throw new Error('The region factory will not generate IDs!');
  },
});
