/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array } from '@ember/helper';
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { gt } from 'ember-truth-helpers';
import AllocationStatusBar from 'nomad-ui/components/allocation-status-bar';
import ChildrenStatusBar from 'nomad-ui/components/children-status-bar';
import JobPagePartsSummaryChart from 'nomad-ui/components/job-page/parts/summary-chart';
import ListAccordion from 'nomad-ui/components/list-accordion';

export default class Summary extends Component {
  @tracked persistedStateVersion = 0;

  get forceCollapsed() {
    return this.args.forceCollapsed ?? false;
  }

  get isExpanded() {
    this.persistedStateVersion;

    if (this.forceCollapsed) {
      return false;
    }

    const storageValue = window.localStorage.nomadExpandJobSummary;
    return storageValue != null ? JSON.parse(storageValue) : true;
  }

  persist = (item, isOpen) => {
    window.localStorage.nomadExpandJobSummary = isOpen;
    this.persistedStateVersion++;
  };

  <template>
    <ListAccordion
      data-test-job-summary
      @source={{array @job}}
      @key="id"
      @startExpanded={{this.isExpanded}}
      @onToggle={{this.persist}}
      as |a|
    >
      <a.head
        @buttonLabel={{if a.isOpen "collapse" "expand"}}
        @buttonType={{if
          a.item.hasChildren
          "children-status"
          "allocation-status"
        }}
      >
        <div class="columns">
          <div class="column is-minimum nowrap">
            {{#if a.item.hasChildren}}
              Children Status
              <span class="badge {{if a.isOpen 'is-white' 'is-light'}}">
                {{a.item.summary.totalChildren}}
              </span>
            {{else}}
              Allocation Status
              <span class="badge {{if a.isOpen 'is-white' 'is-light'}}">
                {{a.item.summary.totalAllocs}}
              </span>
            {{/if}}
          </div>
          {{#unless a.isOpen}}
            <div class="column">
              <div class="inline-chart bumper-left">
                {{#if a.item.hasChildren}}
                  {{#if (gt a.item.totalChildren 0)}}
                    <ChildrenStatusBar
                      @job={{a.item}}
                      @isNarrow={{true}}
                      data-test-children-status-bar
                    />
                  {{else}}
                    <em class="is-faded">
                      No Children
                    </em>
                  {{/if}}
                {{else}}
                  <AllocationStatusBar
                    @allocationContainer={{a.item}}
                    @isNarrow={{true}}
                    data-test-allocation-status-bar
                  />
                {{/if}}
              </div>
            </div>
          {{/unless}}
        </div>
      </a.head>
      <a.body>
        <JobPagePartsSummaryChart @job={{a.item}} />
      </a.body>
    </ListAccordion>
  </template>
}
