/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import WithForbiddenState from 'nomad-ui/mixins/with-forbidden-state';
import notifyForbidden from 'nomad-ui/utils/notify-forbidden';
import classic from 'ember-classic-decorator';

@classic
export default class ServersRoute extends Route.extend(WithForbiddenState) {
  @service store;
  @service system;

  async beforeModel() {
    await this.system.leaders;
  }

  async model() {
    const agents = await this.store.findAll('agent');
    await Promise.all(agents.map((agent) => agent.checkForLeadership()));
    return RSVP.hash({
      nodes: this.store.findAll('node'),
      agents,
    }).catch(notifyForbidden(this));
  }
}
