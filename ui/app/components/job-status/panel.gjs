/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import JobStatusPanelDeploying from 'nomad-ui/components/job-status/panel/deploying';
import JobStatusPanelSteady from 'nomad-ui/components/job-status/panel/steady';

export default class JobStatusPanel extends Component {
  @service store;

  get isActivelyDeploying() {
    return this.args.job.get('latestDeployment.isRunning');
  }

  get nodes() {
    if (!this.args.job.get('hasClientStatus')) {
      return [];
    }

    return this.store.peekAll('node');
  }

  <template>
    {{#if this.isActivelyDeploying}}
      <JobStatusPanelDeploying @job={{@job}} @handleError={{@handleError}} />
    {{else}}
      <JobStatusPanelSteady
        @job={{@job}}
        @statusMode={{@statusMode}}
        @setStatusMode={{@setStatusMode}}
        @nodes={{this.nodes}}
      />
    {{/if}}
  </template>
}
