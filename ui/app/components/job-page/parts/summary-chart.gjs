/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { LinkTo } from '@ember/routing';
import Component from '@glimmer/component';
import { camelize } from '@ember/string';
import { service } from '@ember/service';
import { and, eq, gt } from 'ember-truth-helpers';
import AllocationStatusBar from 'nomad-ui/components/allocation-status-bar';
import ChildrenStatusBar from 'nomad-ui/components/children-status-bar';
import JobPagePartsSummaryLegendItem from 'nomad-ui/components/job-page/parts/summary-legend-item';

export default class JobPagePartsSummaryChart extends Component {
  @service router;

  gotoAllocations = (status) => {
    const namespace = this.args.job.namespaceId || this.args.job.namespace;
    const queryParams = {
      status: JSON.stringify(status),
      page: 1,
      search: '',
      sort: 'modifyIndex',
      desc: true,
      client: '',
      taskGroup: '',
      version: '',
      scheduling: '',
      activeTask: null,
    };

    if (namespace && namespace !== 'default') {
      queryParams.namespace = namespace;
    }

    this.router.transitionTo('jobs.job.allocations', this.args.job, {
      queryParams,
    });
  };

  onSliceClick = (event, slice) => {
    this.gotoAllocations([camelize(slice.label)]);
  };

  <template>
    {{#if @job.hasChildren}}
      <ChildrenStatusBar
        @allocationContainer={{@job.summary}}
        @job={{@job.summary}}
        class="split-view"
        data-test-children-status-bar
        as |chart|
      >
        <ol data-test-legend class="legend">
          {{#each chart.data as |datum index|}}
            <li
              class="{{datum.className}}
                {{if (eq datum.label chart.activeDatum.label) 'is-active'}}
                {{if (eq datum.value 0) 'is-empty'}}"
            >
              <JobPagePartsSummaryLegendItem
                @datum={{datum}}
                @index={{index}}
              />
            </li>
          {{/each}}
        </ol>
      </ChildrenStatusBar>
    {{else}}
      <AllocationStatusBar
        @allocationContainer={{@job.summary}}
        @job={{@job}}
        @onSliceClick={{this.onSliceClick}}
        class="split-view"
        data-test-allocation-status-bar
        as |chart|
      >
        <ol data-test-legend class="legend">
          {{#each chart.data as |datum index|}}
            <li
              data-test-legend-label={{datum.className}}
              class="{{datum.className}}
                {{if (eq datum.label chart.activeDatum.label) 'is-active'}}
                {{if (eq datum.value 0) 'is-empty' 'is-clickable'}}"
            >
              {{#if (and (gt datum.value 0) datum.legendLink)}}
                <LinkTo
                  @route="jobs.job.allocations"
                  @model={{@job}}
                  @query={{datum.legendLink.queryParams}}
                >
                  <JobPagePartsSummaryLegendItem
                    @datum={{datum}}
                    @index={{index}}
                  />
                </LinkTo>
              {{else}}
                <JobPagePartsSummaryLegendItem
                  @datum={{datum}}
                  @index={{index}}
                />
              {{/if}}
            </li>
          {{/each}}
        </ol>
      </AllocationStatusBar>
    {{/if}}
  </template>
}
