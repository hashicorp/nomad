/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';
import { inject as service } from '@ember/service';

export default class ServerRoute extends Route.extend(WithModelErrorHandling) {
  @service store;

  serialize(model) {
    const agentId =
      (typeof model?.get === 'function' ? model.get('id') : undefined) ||
      model?.id ||
      model;

    return { agent_id: agentId };
  }

  model({ agent_id }) {
    return this.store.findRecord('agent', agent_id, { reload: true });
  }
}
