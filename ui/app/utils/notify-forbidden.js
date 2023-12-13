/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// An error handler to provide to a promise catch to set a
// forbidden flag on the route
import codesForError from './codes-for-error';
export default function notifyForbidden(route) {
  return (error) => {
    if (codesForError(error).includes('403')) {
      route.set('isForbidden', true);
    } else {
      throw error;
    }
  };
}
