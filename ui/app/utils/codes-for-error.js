/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// Returns an array of error codes as strings for an Ember error object
export default function codesForError(error) {
  const codes = [error.code];

  if (error.errors) {
    error.errors.forEach((err) => {
      codes.push(err.status);
    });
  }

  return codes
    .compact()
    .uniq()
    .map((code) => '' + code);
}
