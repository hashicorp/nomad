/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

// @ts-check
import { default as ApplicationAdapter, namespace } from './application';

export default class RoleAdapter extends ApplicationAdapter {
  namespace = namespace + '/acl';
}
