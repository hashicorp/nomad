/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { service } from '@ember/service';
import Route from '@ember/routing/route';
import notifyError from 'nomad-ui/utils/notify-error';

export default class ClientRoute extends Route {
  @service store;
  @service router;

  serialize(model) {
    const primitiveModelId =
      typeof model === 'string' || typeof model === 'number'
        ? String(model)
        : undefined;

    const modelId =
      (typeof model?.get === 'function' ? model.get('id') : undefined) ||
      model?.id;

    let currentNodeId;
    try {
      currentNodeId = this.paramsFor('clients.client')?.node_id;
    } catch {
      currentNodeId = undefined;
    }

    // eslint-disable-next-line ember/no-controller-access-in-routes
    const controllerModel = this.controllerFor('clients.client')?.model;
    const controllerNodeId =
      (typeof controllerModel?.get === 'function'
        ? controllerModel.get('id')
        : undefined) || controllerModel?.id;

    let routeModelNodeId;
    try {
      const routeModel = this.modelFor('clients.client');
      routeModelNodeId =
        (typeof routeModel?.get === 'function'
          ? routeModel.get('id')
          : undefined) || routeModel?.id;
    } catch {
      routeModelNodeId = undefined;
    }

    const currentPath = (this.router.currentURL || '').split('?')[0];
    const urlNodeId = currentPath.match(/^\/clients\/([^/]+)/)?.[1];

    return {
      node_id:
        primitiveModelId ||
        modelId ||
        currentNodeId ||
        controllerNodeId ||
        routeModelNodeId ||
        urlNodeId,
    };
  }

  model({ node_id }) {
    return this.store
      .findRecord('node', node_id, { reload: true })
      .catch(notifyError(this));
  }

  afterModel(model) {
    if (model && model.get('isPartial')) {
      return model.reload().then((node) => node.get('allocations'));
    }
    return model && model.get('allocations');
  }
}
