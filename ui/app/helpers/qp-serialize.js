/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';
import { serialize } from 'nomad-ui/utils/qp-serialize';

/**
 * Query Param Serialize
 *
 * Usage: {{qp-serialize array}}
 *
 * Turns an array of values into a safe url encoded query param
 * value. This serialization is used throughout the app for facets.
 */
export function qpSerialize([values]) {
  return serialize(values);
}

export default helper(qpSerialize);
