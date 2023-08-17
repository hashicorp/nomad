/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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
