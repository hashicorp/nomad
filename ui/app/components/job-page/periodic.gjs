/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { fn } from '@ember/helper';
import { on } from '@ember/modifier';
import Component from '@glimmer/component';
import JobPage from 'nomad-ui/components/job-page';
import pluralize from 'nomad-ui/helpers/pluralize';

export default class Periodic extends Component {
  get cronSpecs() {
    return this.args.job?.periodicDetails?.Specs ?? [];
  }

  get cronSpecCount() {
    return this.cronSpecs.length || 1;
  }

  forceLaunch = (setError) => {
    this.args.job.forcePeriodic().catch((err) => {
      setError?.(err);
    });
  };

  <template>
    <JobPage @job={{@job}} as |jobPage|>
      <jobPage.ui.Body>
        <jobPage.ui.Error />
        <jobPage.ui.Title @title={{@job.trimmedName}}>
          <span class="tag is-hollow">
            periodic
          </span>
          <button
            data-test-force-launch
            class="button is-warning is-small is-inline"
            {{on "click" (fn this.forceLaunch jobPage.fns.setError)}}
            type="button"
          >
            Force Launch
          </button>
        </jobPage.ui.Title>
        <jobPage.ui.StatsBox>
          <:afterNamespace>
            <span class="pair" data-test-job-stat="cron">
              <span class="term">
                {{pluralize "Cron" this.cronSpecCount}}
              </span>
              {{#each this.cronSpecs as |spec|}}
                <span class="bumper-right tag">{{spec}}</span>
              {{else}}
                <span class="tag">{{@job.periodicDetails.Spec}}</span>
              {{/each}}
            </span>
          </:afterNamespace>
        </jobPage.ui.StatsBox>
        <jobPage.ui.Summary />
        <jobPage.ui.Children
          @sortProperty={{@sortProperty}}
          @sortDescending={{@sortDescending}}
          @currentPage={{@currentPage}}
          @jobs={{@childJobs}}
        />
        <jobPage.ui.Meta />
      </jobPage.ui.Body>
    </JobPage>
  </template>
}
