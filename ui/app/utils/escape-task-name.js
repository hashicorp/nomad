/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function escapeTaskName(taskName) {
  // Regular expression is taken from here: https://stackoverflow.com/a/20053121
  return taskName.replace(/[^a-zA-Z0-9,._+@%/-]/gu, '\\$&');
}
