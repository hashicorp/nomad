/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { waitForPromise } from '@ember/test-waiters';

const BODY_METHODS = ['json', 'text', 'arrayBuffer', 'blob', 'formData'];

export async function waitForFetch(fetchPromise) {
  waitForPromise(fetchPromise);

  const response = await fetchPromise;

  return new Proxy(response, {
    get(target, prop, receiver) {
      const original = Reflect.get(target, prop, receiver);

      if (BODY_METHODS.includes(prop) && typeof original === 'function') {
        return (...args) => waitForPromise(original.call(target, ...args));
      }

      return original;
    },
  });
}

export function wrappedFetch(...args) {
  return waitForFetch(fetch(...args));
}
