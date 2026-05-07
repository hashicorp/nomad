/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { ForbiddenError } from '@ember-data/adapter/error';

function hasNomadToken() {
  try {
    return Boolean(globalThis?.localStorage?.nomadTokenSecret);
  } catch {
    return false;
  }
}

// Returns a single string based on the response the adapter received
export default function messageFromAdapterError(error, actionMessage) {
  if (error instanceof ForbiddenError) {
    const hasToken = hasNomadToken();
    if (!hasToken) {
      return 'You are not signed in. Please sign in to perform this action.';
    }
    return `Your ACL token does not grant permission to ${actionMessage}.`;
  }

  if (error.errors?.length) {
    return error.errors.mapBy('detail').join('\n\n');
  }

  return 'Unknown Error';
}
