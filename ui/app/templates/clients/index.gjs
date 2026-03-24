/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { fn, get } from '@ember/helper';
import { on } from '@ember/modifier';
import { pageTitle } from 'ember-page-title';
import eq from 'ember-truth-helpers/helpers/eq';
import or from 'ember-truth-helpers/helpers/or';
import {
  filter,
  filterBy,
  includes,
} from '@nullvoxpopuli/ember-composable-helpers';
import { capitalize } from '@ember/string';
import ClientNodeRow from 'nomad-ui/components/client-node-row';
import ForbiddenMessage from 'nomad-ui/components/forbidden-message';
import ListPagination from 'nomad-ui/components/list-pagination';
import ListTable from 'nomad-ui/components/list-table';
import PageSizeSelect from 'nomad-ui/components/page-size-select';
import SearchBox from 'nomad-ui/components/search-box';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';
import {
  HdsDropdown,
  HdsIcon,
  HdsSegmentedGroup,
} from '@hashicorp/design-system-components/components';

<template>
  {{pageTitle "Clients"}}
  <section class="section">
    {{#if @controller.isForbidden}}
      <ForbiddenMessage />
    {{else}}
      <div class="toolbar">
        <div class="toolbar-item">
          {{#if @controller.nodes.length}}
            <SearchBox
              @searchTerm={{@controller.searchTerm}}
              @onChange={{@controller.updateSearchTerm}}
              @placeholder="Search clients..."
            />
          {{/if}}
        </div>

        <HdsSegmentedGroup>
          <HdsDropdown data-test-state-facet as |dd|>
            <dd.ToggleButton
              @text="State"
              @color="secondary"
              @badge={{if
                (eq
                  @controller.activeToggles.length @controller.allToggles.length
                )
                false
                @controller.activeToggles.length
              }}
            />
            <dd.Title @text="Status" />
            {{#each @controller.clientFilterToggles.state as |option|}}
              <dd.Checkbox
                {{on "change" (fn @controller.toggleClientFilter option.qp)}}
                @value={{option.label}}
                @count={{get (filter option.filter @controller.nodes) "length"}}
                checked={{get @controller option.qp}}
                data-test-dropdown-option={{option.label}}
              >
                {{capitalize option.label}}
              </dd.Checkbox>
            {{/each}}
            <dd.Separator />
            <dd.Title @text="Eligibility" />
            {{#each @controller.clientFilterToggles.eligibility as |option|}}
              <dd.Checkbox
                {{on "change" (fn @controller.toggleClientFilter option.qp)}}
                @value={{option.label}}
                @count={{get (filter option.filter @controller.nodes) "length"}}
                checked={{get @controller option.qp}}
                data-test-dropdown-option={{option.label}}
              >
                {{capitalize option.label}}
              </dd.Checkbox>
            {{/each}}
            <dd.Separator />
            <dd.Title @text="Drain Status" />
            {{#each @controller.clientFilterToggles.drainStatus as |option|}}
              <dd.Checkbox
                {{on "change" (fn @controller.toggleClientFilter option.qp)}}
                @value={{option.label}}
                @count={{get (filter option.filter @controller.nodes) "length"}}
                checked={{get @controller option.qp}}
                data-test-dropdown-option={{option.label}}
              >
                {{capitalize option.label}}
              </dd.Checkbox>
            {{/each}}
          </HdsDropdown>

          <HdsDropdown data-test-node-pool-facet as |dd|>
            <dd.ToggleButton
              @text="Node Pool"
              @color="secondary"
              @badge={{or @controller.selectionNodePool.length false}}
            />
            {{#each @controller.optionsNodePool key="label" as |option|}}
              <dd.Checkbox
                {{on
                  "change"
                  (fn
                    @controller.handleFilterChange
                    @controller.selectionNodePool
                    option.label
                    "qpNodePool"
                  )
                }}
                @value={{option.label}}
                checked={{includes option.label @controller.selectionNodePool}}
                @count={{get
                  (filterBy "nodePool" option.label @controller.nodes)
                  "length"
                }}
                data-test-dropdown-option={{option.label}}
              >
                {{option.label}}
              </dd.Checkbox>
            {{else}}
              <dd.Generic data-test-dropdown-empty>
                No Node Pool filters
              </dd.Generic>
            {{/each}}
          </HdsDropdown>

          <HdsDropdown data-test-class-facet as |dd|>
            <dd.ToggleButton
              @text="Class"
              @color="secondary"
              @badge={{or @controller.selectionClass.length false}}
            />
            {{#each @controller.optionsClass key="label" as |option|}}
              <dd.Checkbox
                {{on
                  "change"
                  (fn
                    @controller.handleFilterChange
                    @controller.selectionClass
                    option.label
                    "qpClass"
                  )
                }}
                @value={{option.label}}
                checked={{includes option.label @controller.selectionClass}}
                @count={{get
                  (filterBy "nodeClass" option.label @controller.nodes)
                  "length"
                }}
                data-test-dropdown-option={{option.label}}
              >
                {{option.label}}
              </dd.Checkbox>
            {{else}}
              <dd.Generic data-test-dropdown-empty>
                No Class filters
              </dd.Generic>
            {{/each}}
          </HdsDropdown>

          <HdsDropdown data-test-datacenter-facet as |dd|>
            <dd.ToggleButton
              @text="Datacenter"
              @color="secondary"
              @badge={{or @controller.selectionDatacenter.length false}}
            />
            {{#each @controller.optionsDatacenter key="label" as |option|}}
              <dd.Checkbox
                {{on
                  "change"
                  (fn
                    @controller.handleFilterChange
                    @controller.selectionDatacenter
                    option.label
                    "qpDatacenter"
                  )
                }}
                @value={{option.label}}
                checked={{includes
                  option.label
                  @controller.selectionDatacenter
                }}
                @count={{get
                  (filterBy "datacenter" option.label @controller.nodes)
                  "length"
                }}
                data-test-dropdown-option={{option.label}}
              >
                {{option.label}}
              </dd.Checkbox>
            {{else}}
              <dd.Generic data-test-dropdown-empty>
                No Datacenter filters
              </dd.Generic>
            {{/each}}

          </HdsDropdown>

          <HdsDropdown data-test-version-facet as |dd|>
            <dd.ToggleButton
              @text="Version"
              @color="secondary"
              @badge={{or @controller.selectionVersion.length false}}
            />
            {{#each @controller.optionsVersion key="label" as |option|}}
              <dd.Checkbox
                {{on
                  "change"
                  (fn
                    @controller.handleFilterChange
                    @controller.selectionVersion
                    option.label
                    "qpVersion"
                  )
                }}
                @value={{option.label}}
                checked={{includes option.label @controller.selectionVersion}}
                @count={{get
                  (filterBy "version" option.label @controller.nodes)
                  "length"
                }}
                data-test-dropdown-option={{option.label}}
              >
                {{option.label}}
              </dd.Checkbox>
            {{else}}
              <dd.Generic data-test-dropdown-empty>
                No Version filters
              </dd.Generic>
            {{/each}}
          </HdsDropdown>

          <HdsDropdown data-test-volume-facet as |dd|>
            <dd.ToggleButton
              @text="Volume"
              @color="secondary"
              @badge={{or @controller.selectionVolume.length false}}
            />
            {{#each @controller.optionsVolume key="label" as |option|}}
              <dd.Checkbox
                {{on
                  "change"
                  (fn
                    @controller.handleFilterChange
                    @controller.selectionVolume
                    option.label
                    "qpVolume"
                  )
                }}
                @value={{option.label}}
                checked={{includes option.label @controller.selectionVolume}}
                @count={{get
                  (filterBy "volume" option.label @controller.nodes)
                  "length"
                }}
                data-test-dropdown-option={{option.label}}
              >
                {{option.label}}
              </dd.Checkbox>
            {{else}}
              <dd.Generic data-test-dropdown-empty>
                No Volume filters
              </dd.Generic>
            {{/each}}
          </HdsDropdown>
        </HdsSegmentedGroup>
      </div>
      {{#if @controller.sortedNodes}}
        <ListPagination
          @source={{@controller.sortedNodes}}
          @size={{@controller.pageSize}}
          @page={{@controller.currentPage}}
          as |p|
        >
          <ListTable
            @source={{p.list}}
            @sortProperty={{@controller.sortProperty}}
            @sortDescending={{@controller.sortDescending}}
            @class="with-foot"
            as |t|
          >
            <t.head>
              <th class="is-narrow"><span class="visually-hidden">Driver Health</span></th>
              <t.sortBy @prop="id">ID</t.sortBy>
              <t.sortBy
                @class="is-200px is-truncatable"
                @prop="name"
              >Name</t.sortBy>
              <t.sortBy @prop="status">State</t.sortBy>
              <th class="is-200px is-truncatable">Address</th>
              <t.sortBy @prop="nodePool">Node Pool</t.sortBy>
              <t.sortBy @prop="datacenter">Datacenter</t.sortBy>
              <t.sortBy @prop="version">Version</t.sortBy>
              <th># Volumes</th>
              <th># Allocs</th>
            </t.head>
            <t.body as |row|>
              <ClientNodeRow
                data-test-client-node-row
                @node={{row.model}}
                @onClick={{fn @controller.gotoNode row.model}}
                {{keyboardShortcut
                  enumerated=true
                  action=(fn @controller.gotoNode row.model)
                }}
              />
            </t.body>
          </ListTable>
          <div class="table-foot">
            <PageSizeSelect @onChange={{@controller.resetPagination}} />
            <nav class="pagination" data-test-pagination>
              <div class="pagination-numbers">
                {{p.startsAt}}&ndash;{{p.endsAt}}
                of
                {{@controller.sortedNodes.length}}
              </div>
              <p.prev @class="pagination-previous">
                <HdsIcon @name="chevron-left" @isInline={{true}} />
              </p.prev>
              <p.next @class="pagination-next">
                <HdsIcon @name="chevron-right" @isInline={{true}} />
              </p.next>
              <ul class="pagination-list"></ul>
            </nav>
          </div>
        </ListPagination>
      {{else}}
        <div class="empty-message" data-test-empty-clients-list>
          {{#if (eq @controller.nodes.length 0)}}
            <h3
              class="empty-message-headline"
              data-test-empty-clients-list-headline
            >No Clients</h3>
            <p class="empty-message-body">
              The cluster currently has no client nodes.
            </p>
          {{else if (eq @controller.filteredNodes.length 0)}}
            <h3
              data-test-empty-clients-list-headline
              class="empty-message-headline"
            >No Matches</h3>
            <p class="empty-message-body">
              No clients match your current filter selection.
            </p>
          {{else if @controller.searchTerm}}
            <h3
              class="empty-message-headline"
              data-test-empty-clients-list-headline
            >No Matches</h3>
            <p class="empty-message-body">No clients match the term
              <strong>{{@controller.searchTerm}}</strong></p>
          {{/if}}
        </div>
      {{/if}}
    {{/if}}
  </section>
</template>
