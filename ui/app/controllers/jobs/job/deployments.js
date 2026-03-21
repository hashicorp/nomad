/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import { alias } from '@ember/object';

export default class DeploymentsController extends Controller.extend(
  WithNamespaceResetting,
) {
  @alias('model') job;
}
