/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
export default class VariablesPathRoute extends Route.extend(
  WithForbiddenState
) {
  model({ absolutePath }) {
    if (this.modelFor('variables').errors) {
      notifyForbidden(this)(this.modelFor('variables'));
    } else {
      const treeAtPath =
        this.modelFor('variables').pathTree.findPath(absolutePath);
      if (treeAtPath) {
        return { treeAtPath, absolutePath };
      } else {
        return { absolutePath };
      }
    }
  }
}
