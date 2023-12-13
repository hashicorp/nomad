/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { computed } from '@ember/object';

// An unattractive but robust way to encode query params
export const serialize = (val) => {
  if (typeof val === 'string' || typeof val === 'number') return val;
  return val.length ? JSON.stringify(val) : '';
};

export const deserialize = (str) => {
  try {
    return JSON.parse(str).compact().without('');
  } catch (e) {
    return [];
  }
};

// A computed property macro for deserializing a query param
export const deserializedQueryParam = (qpKey) =>
  computed(qpKey, function () {
    return deserialize(this.get(qpKey));
  });
