/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Fragment from 'ember-data-model-fragments/fragment';
import { fragmentArray } from 'ember-data-model-fragments/attributes';

export default class SidecarProxy extends Fragment {
  @fragmentArray('sidecar-proxy-upstream') upstreams;
}
