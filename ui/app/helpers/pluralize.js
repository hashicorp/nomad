/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from '@ember/component/helper';
import { pluralize } from 'ember-inflector';

export function pluralizeHelper([term, count]) {
  return count === 1 ? term : pluralize(term);
}

export default Helper.helper(pluralizeHelper);
