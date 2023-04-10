/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Controller from '@ember/controller';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

const ALL_NAMESPACE_WILDCARD = '*';

export default class VariablesController extends Controller {
  queryParams = [{ qpNamespace: 'namespace' }];

  @tracked
  qpNamespace = ALL_NAMESPACE_WILDCARD;

  @action
  setNamespace(namespace) {
    this.qpNamespace = namespace;
  }
}
