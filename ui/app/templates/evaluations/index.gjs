/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { concat, fn, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import { pageTitle } from 'ember-page-title';
import didUpdateHelper from 'ember-render-helpers/helpers/did-update-helper';
import eq from 'ember-truth-helpers/helpers/eq';
import EvaluationSidebarDetail from 'nomad-ui/components/evaluation-sidebar/detail';
import ListTable from 'nomad-ui/components/list-table';
import PageSizeSelect from 'nomad-ui/components/page-size-select';
import SearchBox from 'nomad-ui/components/search-box';
import SingleSelectDropdown from 'nomad-ui/components/single-select-dropdown';
import StatusCell from 'nomad-ui/components/status-cell';
import formatMonthTs from 'nomad-ui/helpers/format-month-ts';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';

<template>
  {{pageTitle "Evaluations"}}
  {{didUpdateHelper @controller.notifyEvalChange @controller.currentEval}}

  <section class="section">
    <div class="toolbar">
      <div class="toolbar-item">
        <SearchBox
          data-test-evaluations-search
          @searchTerm={{@controller.searchTerm}}
          @onChange={{@controller.updateSearchTerm}}
          @placeholder="Search evaluations..."
        />
      </div>
      <div class="toolbar-item is-right-aligned">
        <div class="button-bar">
          <SingleSelectDropdown
            data-test-evaluation-namespace-facet
            @label="Namespace"
            @options={{@controller.optionsNamespaces}}
            @selection={{@controller.qpNamespace}}
            @onSelect={{fn @controller.setQueryParam "qpNamespace"}}
          />
          <SingleSelectDropdown
            data-test-evaluation-status-facet
            @label="Status"
            @options={{@controller.optionsEvaluationsStatus}}
            @selection={{@controller.status}}
            @onSelect={{fn @controller.setQueryParam "status"}}
          />
          <SingleSelectDropdown
            data-test-evaluation-triggered-by-facet
            @label="Triggered By"
            @options={{@controller.optionsTriggeredBy}}
            @selection={{@controller.triggeredBy}}
            @onSelect={{fn @controller.setQueryParam "triggeredBy"}}
          />
          <SingleSelectDropdown
            data-test-evaluation-type-facet
            @label="Type"
            @options={{@controller.optionsType}}
            @selection={{@controller.type}}
            @onSelect={{fn @controller.setQueryParam "type"}}
          />
        </div>
      </div>
    </div>

    <div class="table-container">
      {{#if @model.length}}
        <ListTable data-test-eval-table @source={{@model}} as |t|>
          <t.head>
            <th>Evaluation ID</th>
            <th>Resource</th>
            <th>Priority</th>
            <th>Created</th>
            <th>Triggered By</th>
            <th>Status</th>
            <th>Placement Failures</th>
          </t.head>
          <t.body as |row|>
            <tr
              data-test-evaluation={{row.model.shortId}}
              style="cursor: pointer;"
              class={{if (eq @controller.currentEval row.model.id) "is-active"}}
              tabindex="0"
              {{on "click" (fn @controller.handleEvaluationClick row.model)}}
              {{on "keyup" (fn @controller.handleEvaluationClick row.model)}}
            >
              <td
                data-test-id
                {{keyboardShortcut
                  enumerated=true
                  action=(fn
                    @controller.handleEvaluationClick row.model "keynav"
                  )
                }}
              >
                {{row.model.shortId}}
              </td>
              <td data-test-id>
                {{#if row.model.hasJob}}
                  <LinkTo
                    data-test-evaluation-resource
                    @model={{concat
                      row.model.plainJobId
                      "@"
                      row.model.namespace
                    }}
                    @route="jobs.job"
                  >
                    {{row.model.plainJobId}}
                  </LinkTo>
                {{else}}
                  <LinkTo
                    data-test-evaluation-resource
                    @model={{row.model.nodeId}}
                    @route="clients.client"
                  >
                    {{row.model.shortNodeId}}
                  </LinkTo>
                {{/if}}
              </td>
              <td data-test-priority>{{row.model.priority}}</td>
              <td data-test-create-time>{{formatMonthTs
                  row.model.createTime
                }}</td>
              <td data-test-triggered-by>{{row.model.triggeredBy}}</td>
              <td data-test-status class="is-one-line">
                <StatusCell @status={{row.model.status}} />
              </td>
              <td data-test-blocked>
                {{#if (eq row.model.status "blocked")}}
                  N/A - In Progress
                {{else if row.model.hasPlacementFailures}}
                  True
                {{else}}
                  False
                {{/if}}
              </td>
            </tr>
          </t.body>
        </ListTable>

        <div class="table-foot with-padding">
          <PageSizeSelect
            data-test-per-page
            @onChange={{@controller.onChange}}
          />
          <div>
            <button
              class="button"
              data-test-eval-refresh
              type="button"
              {{on "click" @controller.refresh}}
            >
              <HdsIcon @name="reload" @isInline={{true}} />
              Refresh
            </button>
            <button
              data-test-eval-pagination-prev
              type="button"
              class="button is-text is-borderless"
              disabled={{@controller.shouldDisablePrev}}
              {{on "click" @controller.onPrev}}
            >
              <HdsIcon @name="chevron-left" @isInline={{true}} />
            </button>
            <button
              data-test-eval-pagination-next
              type="button"
              class="button is-text is-borderless"
              disabled={{@controller.shouldDisableNext}}
              {{on "click" (fn @controller.onNext @model.meta.nextToken)}}
            >
              <HdsIcon @name="chevron-right" @isInline={{true}} />
            </button>
          </div>
        </div>
      {{else}}
        <div class="boxed-section-body">
          <div class="empty-message" data-test-empty-evaluations-list>
            <h3
              class="empty-message-headline"
              data-test-empty-evalations-list-headline
            >
              No Matches
            </h3>
            <p class="empty-message-body">
              {{#if @controller.hasFiltersApplied}}
                <span data-test-no-eval-match>
                  No evaluations that match:
                  <strong>{{@controller.noMatchText}}</strong>
                </span>
              {{else}}
                <span data-test-no-eval>
                  There are no evaluations
                </span>
              {{/if}}
            </p>
          </div>
        </div>
      {{/if}}
    </div>

    <EvaluationSidebarDetail
      @statechart={{@controller.statechart}}
      @fns={{hash
        closeSidebar=@controller.closeSidebar
        handleEvaluationClick=@controller.handleEvaluationClick
      }}
    />
  </section>
</template>
