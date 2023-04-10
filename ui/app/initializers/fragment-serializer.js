/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import FragmentSerializer from '../serializers/fragment';

export function initialize(application) {
  application.register('serializer:-fragment', FragmentSerializer);
}

export default {
  name: 'fragment-serializer',
  initialize: initialize,
};
