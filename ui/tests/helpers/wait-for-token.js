/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { waitUntil } from '@ember/test-helpers';

export default async function waitForToken(owner) {
  const tokenService = owner.lookup('service:token');
  await waitUntil(() => tokenService.selfToken, { timeout: 2000 });
}
