/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { LinkTo } from '@ember/routing';
import Component from '@glimmer/component';
import { service } from '@ember/service';
import can from 'ember-can/helpers/can';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import ChildJobRow from 'nomad-ui/components/child-job-row';
import ListPagination from 'nomad-ui/components/list-pagination';
import ListTable from 'nomad-ui/components/list-table';
import PageSizeSelect from 'nomad-ui/components/page-size-select';

export default class Children extends Component {
  @service router;
  @service system;
  @service userSettings;

  get pageSize() {
    return this.userSettings.pageSize;
  }

  get sortedChildren() {
    return sortItems(
      this.args.jobs,
      this.args.sortProperty,
      this.args.sortDescending,
    );
  }

  resetPagination = () => {
    if (this.args.currentPage != null && this.router.currentRouteName) {
      this.router.transitionTo({ queryParams: { page: 1 } });
    }
  };

  <template>
    <div class="boxed-section" ...attributes>
      <div class="boxed-section-head">
        Job Launches
        {{#if @job.parameterized}}
          {{#if (can "dispatch job" namespace=@job.namespaceId)}}
            <LinkTo
              data-test-dispatch-button
              @route="jobs.job.dispatch"
              class="button is-primary is-compact pull-right"
            >
              Dispatch Job
            </LinkTo>
          {{else}}
            <button
              data-test-dispatch-button
              class="button is-disabled is-primary is-compact pull-right tooltip multiline"
              aria-label="You don’t have permission to dispatch jobs"
              disabled
              type="button"
            >
              Dispatch Job
            </button>
          {{/if}}
        {{/if}}
      </div>
      <div
        class="boxed-section-body
          {{if this.sortedChildren.length 'is-full-bleed'}}"
      >
        {{#if this.sortedChildren}}
          <ListPagination
            @source={{this.sortedChildren}}
            @size={{this.pageSize}}
            @page={{@currentPage}}
            as |p|
          >
            <ListTable
              @source={{p.list}}
              @sortProperty={{@sortProperty}}
              @sortDescending={{@sortDescending}}
              @class="with-foot"
              as |t|
            >
              <t.head data-test-jobs-header>
                <t.sortBy @prop="name">
                  Name
                </t.sortBy>
                <t.sortBy @prop="submitTime" data-test-jobs-submit-time-header>
                  Submitted At
                </t.sortBy>
                <t.sortBy @prop="status">
                  Status
                </t.sortBy>
                <th class="is-3">
                  Completed Allocations
                </th>
              </t.head>
              <t.body @key="model.id" as |row|>
                <ChildJobRow @job={{row.model}} />
              </t.body>
            </ListTable>
            <div class="table-foot">
              <PageSizeSelect @onChange={{this.resetPagination}} />
              <nav class="pagination">
                <div class="pagination-numbers">
                  {{p.startsAt}}
                  –
                  {{p.endsAt}}
                  of
                  {{this.sortedChildren.length}}
                </div>
                <p.prev @class="pagination-previous">
                  <HdsIcon @name="chevron-left" />
                </p.prev>
                <p.next @class="pagination-next">
                  <HdsIcon @name="chevron-right" />
                </p.next>
                <ul class="pagination-list"></ul>
              </nav>
            </div>
          </ListPagination>
        {{else}}
          <div class="empty-message">
            <h3 class="empty-message-headline">
              No Job Launches
            </h3>
            <p class="empty-message-body">
              No remaining living job launches.
            </p>
          </div>
        {{/if}}
      </div>
    </div>
  </template>
}

function sortItems(items, sortProperty, sortDescending = true) {
  const normalizedItems = (items?.toArray?.() || items || []).filter(Boolean);

  if (!sortProperty) {
    return normalizedItems;
  }

  const sortedItems = normalizedItems
    .slice()
    .sort((left, right) => compareValues(left, right, sortProperty));

  return sortDescending ? sortedItems.reverse() : sortedItems;
}

function compareValues(left, right, sortProperty) {
  const leftValue = getPathValue(left, sortProperty);
  const rightValue = getPathValue(right, sortProperty);

  if (typeof leftValue === 'string' && typeof rightValue === 'string') {
    return leftValue.localeCompare(rightValue);
  }

  if (leftValue === rightValue) {
    return 0;
  }

  if (leftValue == null) {
    return -1;
  }

  if (rightValue == null) {
    return 1;
  }

  return leftValue > rightValue ? 1 : -1;
}

function getPathValue(item, sortProperty) {
  return sortProperty.split('.').reduce((value, key) => value?.[key], item);
}
