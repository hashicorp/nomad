/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import PathTree from 'nomad-ui/utils/path-tree';

export default class VariablesRoute extends Route.extend(WithForbiddenState) {
  @service can;
  @service router;
  @service store;

  queryParams = {
    qpNamespace: {
      refreshModel: true,
    },
  };

  beforeModel() {
    if (this.can.cannot('list variables')) {
      this.router.transitionTo('/jobs');
    }
  }

  async model({ qpNamespace }) {
    const namespace = qpNamespace ?? '*';
    try {
      await this.store.findAll('namespace');
      const variables = await this.store.query(
        'variable',
        { namespace },
        { reload: true }
      );
      return {
        variables,
        pathTree: new PathTree(variables),
      };
    } catch (e) {
      notifyForbidden(this)(e);
      return e;
    }
  }
}
