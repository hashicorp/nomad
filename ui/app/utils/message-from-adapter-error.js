/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { ForbiddenError } from '@ember-data/adapter/error';

// Returns a single string based on the response the adapter received
export default function messageFromAdapterError(error, actionMessage) {
  if (error instanceof ForbiddenError) {
    return `Your ACL token does not grant permission to ${actionMessage}.`;
  }

  if (error.errors?.length) {
    return error.errors.mapBy('detail').join('\n\n');
  }

  return 'Unknown Error';
}
