/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, concat, fn, get, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { capitalize } from '@ember/string';
import perform from 'ember-concurrency/helpers/perform';
import { pageTitle } from 'ember-page-title';
import can from 'ember-can/helpers/can';
import not from 'ember-truth-helpers/helpers/not';
import notEq from 'ember-truth-helpers/helpers/not-eq';
import { filterBy } from '@nullvoxpopuli/ember-composable-helpers';
import ForbiddenMessage from 'nomad-ui/components/forbidden-message';
import JobSearchBox from 'nomad-ui/components/job-search-box';
import JobStatusAllocationStatusRow from 'nomad-ui/components/job-status/allocation-status-row';
import PageSizeSelect from 'nomad-ui/components/page-size-select';
import pluralize from 'nomad-ui/helpers/pluralize';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import {
  HdsAlert,
  HdsApplicationState,
  HdsBadge,
  HdsButton,
  HdsDropdown,
  HdsFormTextInputBase,
  HdsIcon,
  HdsLinkStandalone,
  HdsPageHeader,
  HdsSegmentedGroup,
  HdsTable,
  HdsTooltipButton,
} from '@hashicorp/design-system-components/components';
import autofocus from 'nomad-ui/modifiers/autofocus';

<template>
  {{pageTitle "Jobs"}}
  <section class="section">
    {{#if @controller.showingCachedJobs}}
      <HdsAlert
        @type="inline"
        @color="warning"
        id="jobs-list-cache-warning"
        as |A|
      >
        <A.Title>Error fetching jobs — shown jobs are cached</A.Title>
        <A.Description>Jobs shown are cached and may be out of date. This is
          often due to a short timeout in proxy configurations.</A.Description>
        {{#if @controller.watchJobIDs.isRunning}}
          <A.Button
            data-test-pause-fetching
            @text="Stop polling for job updates"
            @color="secondary"
            {{on "click" @controller.pauseJobFetching}}
          />
        {{/if}}
        <A.Button
          data-test-restart-fetching
          @text="Manually fetch jobs"
          @color="secondary"
          {{on "click" @controller.restartJobList}}
        />
        <A.LinkStandalone
          @size="medium"
          @color="primary"
          @icon="learn-link"
          @iconPosition="trailing"
          @text="Tutorial: Configure reverse proxy for Nomad's web UI"
          @href="https://developer.hashicorp.com/nomad/docs/deploy/clusters/reverse-proxy-ui#extend-connection-timeout"
        />
      </HdsAlert>
    {{/if}}

    <HdsPageHeader id="jobs-list-header" as |PH|>
      <PH.Actions id="jobs-list-actions">

        <HdsSegmentedGroup>

          <JobSearchBox
            @searchText={{@controller.rawSearchText}}
            @onSearchTextChange={{@controller.onSearchTextChange}}
          />

          {{#each @controller.filterFacets as |group|}}
            <HdsDropdown
              data-test-facet={{group.label}}
              @height="300px"
              as |dd|
            >
              <dd.ToggleButton
                @text={{group.label}}
                @color="secondary"
                @count={{get (filterBy "checked" true group.options) "length"}}
              />
              {{#if group.filterable}}
                <dd.Header @hasDivider={{true}}>
                  <HdsFormTextInputBase
                    @type="search"
                    placeholder="Filter {{group.label}}"
                    @value={{group.filter}}
                    {{autofocus}}
                    {{on "input" (fn @controller.updateFacetFilter group)}}
                    data-test-namespace-filter-searchbox
                  />
                </dd.Header>
                {{#each
                  (get
                    @controller
                    (concat "filtered" (capitalize group.label) "Options")
                  )
                  as |option|
                }}
                  <dd.Checkbox
                    {{on "change" (fn @controller.toggleOption option)}}
                    @value={{option.key}}
                    checked={{option.checked}}
                    data-test-dropdown-option={{option.key}}
                  >
                    {{option.key}}
                  </dd.Checkbox>
                {{else}}
                  <dd.Generic data-test-dropdown-empty>
                    No
                    {{group.label}}
                    filters match
                    {{group.filter}}
                  </dd.Generic>
                {{/each}}
              {{else}}
                {{#each group.options as |option|}}
                  <dd.Checkbox
                    {{on "change" (fn @controller.toggleOption option)}}
                    @value={{option.key}}
                    checked={{option.checked}}
                    data-test-dropdown-option={{option.key}}
                  >
                    {{option.key}}
                  </dd.Checkbox>
                {{else}}
                  <dd.Generic data-test-dropdown-empty>
                    No
                    {{group.label}}
                    filters
                  </dd.Generic>
                {{/each}}
              {{/if}}
            </HdsDropdown>
          {{/each}}

          {{#if @controller.filter}}
            <HdsButton
              @text="Reset Filters"
              @color="critical"
              @icon="delete"
              @size="medium"
              {{on "click" @controller.resetFilters}}
            />
          {{/if}}

        </HdsSegmentedGroup>

        {{#if @controller.pendingJobIDDiff}}
          <HdsButton
            @size="medium"
            @text="Updates Pending"
            @color="primary"
            @icon="sync"
            {{on "click" (perform @controller.updateJobList)}}
            data-test-updates-pending-button
          />
        {{/if}}

        <div
          {{keyboardShortcut
            label="Run Job"
            pattern=(array "r" "u" "n")
            action=@controller.goToRun
          }}
        >
          <HdsButton
            data-test-run-job
            @text="Run Job"
            disabled={{not (can "run job")}}
            title={{if
              (can "run job")
              null
              "You don’t have permission to run jobs"
            }}
            @route="jobs.run"
          />
        </div>

      </PH.Actions>
    </HdsPageHeader>

    {{#if @controller.isForbidden}}
      <ForbiddenMessage />
    {{else if @controller.jobs.length}}
      <HdsTable
        @model={{@controller.jobs}}
        @columns={{@controller.tableColumns}}
        @valign="middle"
      >
        <:body as |B|>
          {{! TODO: use <JobRow> }}
          <B.Tr
            {{keyboardShortcut
              enumerated=true
              action=(fn @controller.gotoJob B.data)
            }}
            {{on "click" (fn @controller.gotoJob B.data)}}
            class="job-row is-interactive {{if B.data.assumeGC 'assume-gc'}}"
            data-test-job-row={{B.data.plainId}}
            data-test-modify-index={{B.data.modifyIndex}}
          >
            <B.Td data-test-job-name>
              {{#if B.data.assumeGC}}
                {{B.data.name}}
              {{else}}
                <LinkTo
                  @route="jobs.job.index"
                  @model={{B.data.idWithNamespace}}
                  class="is-primary"
                >
                  {{B.data.name}}
                  {{#if B.data.isPack}}
                    <span data-test-pack-tag class="tag is-pack">
                      <HdsIcon @name="box" @color="faint" />
                      <span>Pack</span>
                    </span>
                  {{/if}}
                </LinkTo>
              {{/if}}
            </B.Td>
            {{#if @controller.system.shouldShowNamespaces}}
              <B.Td data-test-job-namespace>{{B.data.namespace.id}}</B.Td>
            {{/if}}
            <B.Td data-test-job-status>
              <div class="status-cell">
                {{#if (notEq B.data.childStatuses null)}}
                  {{#if B.data.childStatusBreakdown.running}}
                    <HdsBadge
                      @icon="corner-down-right"
                      @text="{{B.data.childStatusBreakdown.running}} running {{pluralize
                        'job'
                        B.data.childStatusBreakdown.running
                      }}"
                      @color="highlight"
                      @size="large"
                    />
                  {{else if B.data.childStatusBreakdown.pending}}
                    <HdsBadge
                      @icon="corner-down-right"
                      @text="{{B.data.childStatusBreakdown.pending}} pending {{pluralize
                        'job'
                        B.data.childStatusBreakdown.pending
                      }}"
                      @color="neutral"
                      @size="large"
                    />
                  {{else if B.data.childStatusBreakdown.dead}}
                    <HdsBadge
                      @icon="corner-down-right"
                      @text="{{B.data.childStatusBreakdown.dead}} completed {{pluralize
                        'job'
                        B.data.childStatusBreakdown.dead
                      }}"
                      @color="neutral"
                      @size="large"
                    />
                  {{else if (not B.data.childStatuses.length)}}
                    <HdsBadge
                      @icon="corner-down-right"
                      @text="No child jobs"
                      @color="neutral"
                      @size="large"
                    />
                  {{/if}}
                {{else}}
                  <HdsBadge
                    @text="{{capitalize B.data.aggregateAllocStatus.label}}"
                    @color={{B.data.aggregateAllocStatus.state}}
                    @size="large"
                  />
                {{/if}}
                {{#if B.data.hasPausedTask}}
                  <HdsTooltipButton
                    @text="At least one task is paused"
                    @placement="right"
                    data-test-paused-task-indicator
                  >
                    <HdsBadge
                      @text="At least one task is paused"
                      @icon="delay"
                      @isIconOnly={{true}}
                      @color="highlight"
                      @size="large"
                    />
                  </HdsTooltipButton>
                {{/if}}
              </div>
            </B.Td>
            <B.Td data-test-job-type={{B.data.type}}>
              {{B.data.type}}
            </B.Td>
            {{#if @controller.system.shouldShowNodepools}}
              <B.Td data-test-job-node-pool>{{B.data.nodePool}}</B.Td>
            {{/if}}
            <B.Td>
              <div class="job-status-panel compact">
                {{#unless B.data.assumeGC}}
                  {{#if (notEq B.data.childStatuses null)}}
                    {{#if B.data.childStatuses.length}}
                      <HdsLinkStandalone
                        @icon="chevron-right"
                        @text="View {{B.data.childStatuses.length}} child {{pluralize
                          'job'
                          B.data.childStatuses.length
                        }}"
                        @route="jobs.job.index"
                        @model={{B.data.idWithNamespace}}
                        @iconPosition="trailing"
                      />
                    {{else}}
                      --
                    {{/if}}
                  {{else}}
                    <JobStatusAllocationStatusRow
                      @allocBlocks={{B.data.allocBlocks}}
                      @steady={{true}}
                      @compact={{true}}
                      @runningAllocs={{B.data.allocBlocks.running.healthy.nonCanary.length}}
                      @groupCountSum={{B.data.expectedRunningAllocCount}}
                    />
                  {{/if}}
                {{/unless}}
              </div>
            </B.Td>
          </B.Tr>
        </:body>
      </HdsTable>

      <section id="jobs-list-pagination">
        <div class="nav-buttons">
          <span
            {{keyboardShortcut
              label="First Page"
              pattern=(array "{" "{")
              action=(fn @controller.handlePageChange "first")
            }}
          >
            <HdsButton
              @text="First"
              @color="tertiary"
              @size="small"
              @icon="chevrons-left"
              @iconPosition="leading"
              disabled={{not @controller.cursorAt}}
              data-test-pager="first"
              {{on "click" (fn @controller.handlePageChange "first")}}
            />
          </span>
          <span
            {{keyboardShortcut
              label="Previous Page"
              pattern=(array "[" "[")
              action=(fn @controller.handlePageChange "prev")
            }}
          >
            <HdsButton
              @text="Previous"
              @color="tertiary"
              @size="small"
              @icon="chevron-left"
              @iconPosition="leading"
              disabled={{not @controller.cursorAt}}
              data-test-pager="previous"
              {{on "click" (fn @controller.handlePageChange "prev")}}
            />
          </span>
          <span
            {{keyboardShortcut
              label="Next Page"
              pattern=(array "]" "]")
              action=(fn @controller.handlePageChange "next")
            }}
          >
            <HdsButton
              @text="Next"
              @color="tertiary"
              @size="small"
              @icon="chevron-right"
              @iconPosition="trailing"
              disabled={{not @controller.nextToken}}
              data-test-pager="next"
              {{on "click" (fn @controller.handlePageChange "next")}}
            />
          </span>
          <span
            {{keyboardShortcut
              label="Last Page"
              pattern=(array "}" "}")
              action=(fn @controller.handlePageChange "last")
            }}
          >
            <HdsButton
              @text="Last"
              @color="tertiary"
              @icon="chevrons-right"
              @iconPosition="trailing"
              @size="small"
              disabled={{not @controller.nextToken}}
              data-test-pager="last"
              {{on "click" (fn @controller.handlePageChange "last")}}
            />
          </span>
        </div>
        <div class="page-size">
          <PageSizeSelect @onChange={{@controller.handlePageSizeChange}} />
        </div>
      </section>
    {{else}}
      <HdsApplicationState data-test-empty-jobs-list as |A|>
        {{#if @controller.filter}}
          <A.Header data-test-empty-jobs-list-headline @title="No Matches" />
          <A.Body>
            {{@controller.humanizedFilterError}}
            <br /><br />
            {{#if @model.error.correction}}
              Did you mean
              <HdsButton
                data-test-filter-correction
                @text={{@model.error.correction.correctKey}}
                @isInline={{true}}
                @color="tertiary"
                @icon="bulb"
                @iconPosition="trailing"
                {{on
                  "click"
                  (fn @controller.correctFilterKey @model.error.correction)
                }}
              />?
            {{else if @model.error.suggestion}}
              <ul>
                {{#each @model.error.suggestion as |suggestion|}}
                  <li><HdsButton
                      data-test-filter-suggestion
                      @text={{suggestion.key}}
                      @isInline={{true}}
                      @size="small"
                      @color="tertiary"
                      @icon="bulb"
                      {{on "click" (fn @controller.suggestFilter suggestion)}}
                    /></li>
                {{/each}}
              </ul>
            {{else}}
              {{! This is the "Nothing was found for your otherwise valid filter" option. Give them suggestions }}
              Did you know: you can try using filter expressions to search
              through your jobs. Try
              <HdsButton
                data-test-filter-random-suggestion
                @text={{@controller.exampleFilter}}
                @isInline={{true}}
                @color="tertiary"
                @icon="bulb"
                @iconPosition="trailing"
                {{on
                  "click"
                  (fn
                    @controller.suggestFilter
                    (hash example=@controller.exampleFilter)
                  )
                }}
              />
            {{/if}}
          </A.Body>
          <A.Footer as |F|>
            <HdsButton
              @text="Reset Filters"
              @color="tertiary"
              @icon="arrow-left"
              @size="small"
              {{on "click" @controller.resetFilters}}
            />

            <F.LinkStandalone
              @icon="docs"
              @text="Learn more about Filter Expressions"
              @href="https://developer.hashicorp.com/nomad/api-docs#creating-expressions"
              @iconPosition="trailing"
            />

            <F.LinkStandalone
              @icon="plus"
              @text="Run a New Job"
              @route="jobs.run"
              @iconPosition="trailing"
            />

          </A.Footer>
        {{else}}
          <A.Header data-test-empty-jobs-list-headline @title="No Jobs" />
          <A.Body @text="No jobs found." />
          <A.Footer as |F|>
            <F.LinkStandalone
              @icon="plus"
              @text="Run a New Job"
              @route="jobs.run"
              @iconPosition="trailing"
            />
          </A.Footer>
        {{/if}}
      </HdsApplicationState>
    {{/if}}
  </section>
</template>
