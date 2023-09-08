/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';

export default class VariablesIndexRoute extends Route.extend(
  WithForbiddenState
) {
  model() {
    if (this.modelFor('variables').errors) {
      notifyForbidden(this)(this.modelFor('variables'));
    } else {
      const { variables, pathTree } = this.modelFor('variables');
      return {
        variables,
        root: pathTree.paths.root,
      };
    }
  }
}
