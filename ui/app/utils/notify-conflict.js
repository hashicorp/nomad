/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
// Catches errors with conflicts (409)
// and allow the route to handle them.
import { set } from '@ember/object';
import codesForError from './codes-for-error';
export default function notifyConflict(parent) {
  return (error) => {
    if (codesForError(error).includes('409')) {
      set(parent, 'hasConflict', true);
    } else {
      return error;
    }
  };
}
