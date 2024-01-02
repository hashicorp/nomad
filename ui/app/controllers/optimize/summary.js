/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as controller } from '@ember/controller';

export default class OptimizeSummaryController extends Controller {
  @controller('optimize') optimizeController;

  queryParams = [
    {
      jobNamespace: 'namespace',
    },
  ];

  get summary() {
    return this.model;
  }

  get breadcrumb() {
    const { slug } = this.summary;
    return {
      label: slug.replace('/', ' / '),
      args: ['optimize.summary', slug],
    };
  }
}
