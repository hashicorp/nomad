/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';

export default class StructuredAttributes extends ApplicationSerializer {
  normalize(typeHash, hash) {
    return super.normalize(typeHash, { Raw: hash });
  }
}
