/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { fn } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import { on } from '@ember/modifier';
import { service } from '@ember/service';
import { eq, gt, notEq } from 'ember-truth-helpers';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import formatMonthTs from 'nomad-ui/helpers/format-month-ts';
import AllocationStatusBar from 'nomad-ui/components/allocation-status-bar';
import ChildrenStatusBar from 'nomad-ui/components/children-status-bar';
import { lazyClick } from 'nomad-ui/helpers/lazy-click';

export default class JobRow extends Component {
  @service router;
  @service system;

  click = (event) => {
    lazyClick([this.gotoJob, event]);
  };

  gotoJob = () => {
    const { job } = this.args;
    this.router.transitionTo('jobs.job.index', job.idWithNamespace);
  };

  <template>
    <tr
      class="job-row is-interactive"
      data-test-job-row
      {{on "click" this.click}}
      ...attributes
    >
      <td
        data-test-job-name
        {{keyboardShortcut enumerated=true action=(fn this.gotoJob @job)}}
      >
        <LinkTo
          @route="jobs.job.index"
          @model={{@job.idWithNamespace}}
          class="is-primary"
        >
          {{@job.name}}

          {{#if @job.meta.structured.pack}}
            <span data-test-pack-tag class="tag is-pack">
              <span>Pack</span>
            </span>
          {{/if}}

        </LinkTo>
      </td>
      {{#if (notEq @context "child")}}
        {{#if this.system.shouldShowNamespaces}}
          <td data-test-job-namespace>
            {{@job.namespace.name}}
          </td>
        {{/if}}
      {{/if}}
      {{#if (eq @context "child")}}
        <td data-test-job-submit-time>
          {{formatMonthTs @job.submitTime}}
        </td>
      {{/if}}
      <td data-test-job-status>
        <span class="tag {{@job.statusClass}}">
          {{@job.status}}
        </span>
      </td>
      {{#if (notEq @context "child")}}
        <td data-test-job-type>
          {{@job.displayType.type}}
        </td>
        <td data-test-job-node-pool>
          {{#if @job.nodePool}}{{@job.nodePool}}{{else}}-{{/if}}
        </td>
        <td data-test-job-priority>
          {{@job.priority}}
        </td>
      {{/if}}
      <td data-test-job-allocations>
        <div class="inline-chart">
          {{#if @job.hasChildren}}
            {{#if (gt @job.totalChildren 0)}}
              <ChildrenStatusBar @job={{@job}} @isNarrow={{true}} />
            {{else}}
              <em class="is-faded">
                No Children
              </em>
            {{/if}}
          {{else}}
            <AllocationStatusBar
              @allocationContainer={{@job}}
              @isNarrow={{true}}
            />
          {{/if}}
        </div>
      </td>
    </tr>
  </template>
}
