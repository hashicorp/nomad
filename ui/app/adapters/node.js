/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Watchable from './watchable';
import addToPath from 'nomad-ui/utils/add-to-path';
import classic from 'ember-classic-decorator';

@classic
export default class NodeAdapter extends Watchable {
  setEligible(node) {
    return this.setEligibility(node, true);
  }

  setIneligible(node) {
    return this.setEligibility(node, false);
  }

  setEligibility(node, isEligible) {
    const url = addToPath(
      this.urlForFindRecord(node.id, 'node'),
      '/eligibility'
    );
    return this.ajax(url, 'POST', {
      data: {
        NodeID: node.id,
        Eligibility: isEligible ? 'eligible' : 'ineligible',
      },
    });
  }

  // Force: -1s deadline
  // No Deadline: 0 deadline
  drain(node, drainSpec) {
    const url = addToPath(this.urlForFindRecord(node.id, 'node'), '/drain');
    return this.ajax(url, 'POST', {
      data: {
        NodeID: node.id,
        DrainSpec: Object.assign(
          {
            Deadline: 0,
            IgnoreSystemJobs: true,
          },
          drainSpec
        ),
      },
    });
  }

  forceDrain(node, drainSpec) {
    return this.drain(
      node,
      Object.assign({}, drainSpec, {
        Deadline: -1,
      })
    );
  }

  cancelDrain(node) {
    const url = addToPath(this.urlForFindRecord(node.id, 'node'), '/drain');
    return this.ajax(url, 'POST', {
      data: {
        NodeID: node.id,
        DrainSpec: null,
      },
    });
  }

  addMeta(node, newMeta) {
    const url = `/v1/client/metadata?node_id=${node.id}`;
    return this.ajax(url, 'POST', {
      data: { Meta: newMeta },
    });
  }
}
