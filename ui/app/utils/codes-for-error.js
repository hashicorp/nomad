/**
 * Copyright IBM Corp. 2015, 2025
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

  return [...new Set(codes.filter(Boolean))].map((code) => '' + code);
}
