/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';

export default class DeploymentsController extends Controller.extend(
  WithNamespaceResetting,
) {
  get job() {
    return this.model;
  }
}
