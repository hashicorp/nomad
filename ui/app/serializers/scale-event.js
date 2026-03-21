/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import ApplicationSerializer from './application';

export default class ScaleEventSerializer extends ApplicationSerializer {
  separateNanos = ['Time'];
  objectNullOverrides = ['Meta'];
}
