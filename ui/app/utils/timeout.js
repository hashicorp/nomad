/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import RSVP from 'rsvp';

// An always failing promise used to race against other promises
export default function timeout(duration) {
  return new RSVP.Promise((resolve, reject) => {
    setTimeout(() => {
      reject(`Timeout of ${duration}ms exceeded`);
    }, duration);
  });
}
