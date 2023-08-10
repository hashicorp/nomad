/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { copy } from 'ember-copy';

// Used with fetch.
// Fetch only goes into the promise catch if there is a network error.
// This means that handling a 4xx or 5xx error is the responsibility
// of the developer.
const jsonWithDefault = (defaultResponse) => (res) =>
  res.ok ? res.json() : copy(defaultResponse, true);

export default jsonWithDefault;
