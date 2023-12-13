/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';
import { deserialize } from 'nomad-ui/utils/qp-serialize';

/**
 * Query Param Serialize
 *
 * Usage: {{qp-deserialize string}}
 *
 * Turns a serialized query param value string back into
 * an array of values.
 */
export function qpDeserialize([str]) {
  return deserialize(str);
}

export default helper(qpDeserialize);
