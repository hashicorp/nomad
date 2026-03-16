/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { getContext } from '@ember/test-helpers';
import startMirageInternal from 'ember-cli-mirage/start-mirage';

export function startMirage(options) {
  const context = getContext();

  if (!context?.owner) {
    throw new Error(
      'startMirage() requires an owner. Call setupTest/setupRenderingTest/setupApplicationTest before using it.',
    );
  }

  return startMirageInternal(context.owner, options);
}
