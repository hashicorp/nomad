/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Controller from '@ember/controller';

export default class ClientMonitorController extends Controller {
  queryParams = ['level'];
  level = 'info';
}
