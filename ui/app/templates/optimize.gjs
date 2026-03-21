/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, fn, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import eq from 'ember-truth-helpers/helpers/eq';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import DasRecommendationRow from 'nomad-ui/components/das/recommendation-row';
import ListTable from 'nomad-ui/components/list-table';
import MultiSelectDropdown from 'nomad-ui/components/multi-select-dropdown';
import PageLayout from 'nomad-ui/components/page-layout';
import SearchBox from 'nomad-ui/components/search-box';
import SingleSelectDropdown from 'nomad-ui/components/single-select-dropdown';
import pluralize from 'nomad-ui/helpers/pluralize';

<template>
  <Breadcrumb @crumb={{hash label="Recommendations" args=(array "optimize")}} />
  <PageLayout>
    <section class="section">
      {{#if @controller.summaries}}
        <div class="toolbar collapse">
          <div class="toolbar-item">
            {{#if @controller.summaries}}
              <SearchBox
                data-test-recommendation-summaries-search
                @onChange={{@controller.updateSearchTerm}}
                @searchTerm={{@controller.searchTerm}}
                @placeholder="Search
        {{@controller.summaries.length}}
        {{pluralize 'recommendation' @controller.summaries.length}}..."
              />
            {{/if}}
          </div>
          <div class="toolbar-item is-right-aligned is-mobile-full-width">
            <div class="button-bar">
              {{#if @controller.system.shouldShowNamespaces}}
                <SingleSelectDropdown
                  data-test-namespace-facet
                  @label="Namespace"
                  @options={{@controller.optionsNamespaces}}
                  @selection={{@controller.qpNamespace}}
                  @onSelect={{fn @controller.setFacetQueryParam "qpNamespace"}}
                />
              {{/if}}
              <MultiSelectDropdown
                data-test-type-facet
                @label="Type"
                @options={{@controller.optionsType}}
                @selection={{@controller.selectionType}}
                @onSelect={{fn @controller.setFacetQueryParam "qpType"}}
              />
              <MultiSelectDropdown
                data-test-status-facet
                @label="Status"
                @options={{@controller.optionsStatus}}
                @selection={{@controller.selectionStatus}}
                @onSelect={{fn @controller.setFacetQueryParam "qpStatus"}}
              />
              <MultiSelectDropdown
                data-test-datacenter-facet
                @label="Datacenter"
                @options={{@controller.optionsDatacenter}}
                @selection={{@controller.selectionDatacenter}}
                @onSelect={{fn @controller.setFacetQueryParam "qpDatacenter"}}
              />
              <MultiSelectDropdown
                data-test-prefix-facet
                @label="Prefix"
                @options={{@controller.optionsPrefix}}
                @selection={{@controller.selectionPrefix}}
                @onSelect={{fn @controller.setFacetQueryParam "qpPrefix"}}
              />
            </div>
          </div>
        </div>

        {{#if @controller.filteredSummaries}}
          {{outlet}}

          <ListTable @source={{@controller.filteredSummaries}} as |t|>
            <t.head>
              <th>Job</th>
              <th>Recommended At</th>
              <th># Allocs</th>
              <th>CPU</th>
              <th>Mem</th>
              <th>Agg. CPU</th>
              <th>Agg. Mem</th>
            </t.head>
            <t.body as |row|>
              {{#if row.model.isProcessed}}
                <DasRecommendationRow
                  class="is-disabled"
                  @summary={{row.model}}
                />
              {{else}}
                <DasRecommendationRow
                  class="is-interactive
                    {{if
                      (eq row.model @controller.activeRecommendationSummary)
                      'is-active'
                    }}"
                  @summary={{row.model}}
                  {{on "click" (fn @controller.transitionToSummary row.model)}}
                />
              {{/if}}
            </t.body>
          </ListTable>
        {{else}}
          <div class="empty-message" data-test-empty-recommendations>
            <h3
              class="empty-message-headline"
              data-test-empty-recommendations-headline
            >
              No Matches
            </h3>
            <p class="empty-message-body">
              No recommendations match your current filter selection.
            </p>
          </div>
        {{/if}}
      {{else}}
        <div class="empty-message" data-test-empty-recommendations>
          <h3
            class="empty-message-headline"
            data-test-empty-recommendations-headline
          >
            No Recommendations
          </h3>
          <p class="empty-message-body">
            All recommendations have been accepted or dismissed. Nomad will
            continuously monitor applications so expect more recommendations in
            the future.
          </p>
        </div>
      {{/if}}
    </section>
  </PageLayout>
</template>
