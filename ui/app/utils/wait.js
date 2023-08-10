/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import RSVP from 'rsvp';

// An always passing promise used to throttle other promises
export default function wait(duration) {
  return new RSVP.Promise((resolve) => {
    setTimeout(() => {
      resolve(`Waited ${duration}ms`);
    }, duration);
  });
}
