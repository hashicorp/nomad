/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { LinkTo } from '@ember/routing';
import Component from '@glimmer/component';
import JobPage from 'nomad-ui/components/job-page';
import JsonViewer from 'nomad-ui/components/json-viewer';

export default class ParameterizedChild extends Component {
  get payload() {
    return this.args.job?.decodedPayload;
  }

  get payloadJSON() {
    let json;

    try {
      json = JSON.parse(this.payload);
    } catch {
      // Swallow error and fall back to plain text rendering.
    }

    return json;
  }

  <template>
    <JobPage @job={{@job}} as |jobPage|>
      <jobPage.ui.Body>
        <jobPage.ui.Error />
        <jobPage.ui.Title @title={{@job.trimmedName}} />
        <jobPage.ui.StatsBox>
          <:beforeNamespace>
            <span class="pair" data-test-job-stat="parent">
              <span class="term">
                Parent
              </span>
              <LinkTo @route="jobs.job" @model={{@job.parent}}>
                {{@job.parent.name}}
              </LinkTo>
            </span>
          </:beforeNamespace>
        </jobPage.ui.StatsBox>
        <jobPage.ui.PlacementFailures />
        <jobPage.ui.StatusPanel
          @statusMode={{@statusMode}}
          @setStatusMode={{@setStatusMode}}
        />
        <jobPage.ui.TaskGroups
          @sortProperty={{@sortProperty}}
          @sortDescending={{@sortDescending}}
        />
        <jobPage.ui.RecentAllocations
          @activeTask={{@activeTask}}
          @setActiveTaskQueryParam={{@setActiveTaskQueryParam}}
        />
        <div class="boxed-section">
          {{#if @job.meta}}
            <jobPage.ui.Meta />
          {{else}}
            <div class="boxed-section-head">
              Meta
            </div>
            <div class="boxed-section-body">
              <div data-test-empty-meta-message class="empty-message">
                <h3 class="empty-message-headline">
                  No Meta Attributes
                </h3>
                <p class="empty-message-body">
                  This job is configured with no meta attributes.
                </p>
              </div>
            </div>
          {{/if}}
        </div>
        <div class="boxed-section">
          <div class="boxed-section-head">
            Payload
          </div>
          <div class="boxed-section-body is-dark">
            {{#if this.payloadJSON}}
              <JsonViewer @json={{this.payloadJSON}} />
            {{else}}
              <pre class="cli-window is-elastic">
                <code>
                  {{this.payload}}
                </code>
              </pre>
            {{/if}}
          </div>
        </div>
      </jobPage.ui.Body>
    </JobPage>
  </template>
}
