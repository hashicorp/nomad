/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { array, hash } from '@ember/helper';
import { on } from '@ember/modifier';
import { LinkTo } from '@ember/routing';
import { pageTitle } from 'ember-page-title';
import cannot from 'ember-can/helpers/cannot';
import eq from 'ember-truth-helpers/helpers/eq';
import Breadcrumb from 'nomad-ui/components/breadcrumb';
import ForbiddenMessage from 'nomad-ui/components/forbidden-message';
import MultiSelectDropdown from 'nomad-ui/components/multi-select-dropdown';
import PageLayout from 'nomad-ui/components/page-layout';
import PrimaryMetricAllocation from 'nomad-ui/components/primary-metric/allocation';
import SearchBox from 'nomad-ui/components/search-box';
import TopoViz from 'nomad-ui/components/topo-viz';
import formatBytes from 'nomad-ui/helpers/format-bytes';
import formatHertz from 'nomad-ui/helpers/format-hertz';
import formatPercentage from 'nomad-ui/helpers/format-percentage';
import formatScheduledBytes from 'nomad-ui/helpers/format-scheduled-bytes';
import formatScheduledHertz from 'nomad-ui/helpers/format-scheduled-hertz';
import pluralize from 'nomad-ui/helpers/pluralize';

<template>
  <Breadcrumb @crumb={{hash label="Topology" args=(array "topology")}} />
  {{pageTitle "Cluster Topology"}}
  <PageLayout>
    <section class="section is-full-width">
      {{#if @controller.isForbidden}}
        <ForbiddenMessage />
      {{else}}
        {{#if @controller.pre09Nodes}}
          <div class="notification is-warning">
            <div data-test-filtered-nodes-warning class="columns">
              <div class="column">
                <h3 data-test-title class="title is-4">
                  Some Clients Were Filtered
                </h3>
                <p data-test-message>
                  {{@controller.pre09Nodes.length}}
                  {{if
                    (eq @controller.pre09Nodes.length 1)
                    "client was"
                    "clients were"
                  }}
                  filtered from the topology visualization. This is most likely
                  due to the
                  {{pluralize "client" @controller.pre09Nodes.length}}
                  running a version of Nomad
                </p>
              </div>
              <div class="column is-centered is-minimum">
                <button
                  data-test-dismiss
                  class="button is-warning"
                  {{on "click" @controller.dismissFilteredNodesWarning}}
                  type="button"
                >
                  Okay
                </button>
              </div>
            </div>
          </div>
        {{/if}}
        <div class="columns">
          <div class="column is-narrow is-400">
            {{#if @controller.showPollingNotice}}
              <div class="notification is-warning">
                <div class="columns">
                  <div class="column">
                    <h3 class="title is-4">
                      No Live Updating
                    </h3>
                    <p>
                      The topology visualization depends on too much data to
                      continuously poll.
                    </p>
                  </div>
                  <div class="column is-centered is-minimum">
                    <button
                      data-test-polling-notice-dismiss
                      class="button is-warning"
                      type="button"
                      {{on "click" @controller.dismissPollingNotice}}
                    >
                      Okay
                    </button>
                  </div>
                </div>
              </div>
            {{/if}}
            <div class="boxed-section">
              <div class="boxed-section-head">
                Legend
                {{#if (cannot "list all jobs")}}
                  <span
                    aria-label="Your ACL token may limit your ability to list all allocations"
                    class="tag is-warning pull-right tooltip multiline"
                  >
                    Partial View
                  </span>
                {{/if}}
              </div>
              <div class="boxed-section-body">
                <div class="legend">
                  <h3 class="legend-label">
                    Metrics
                  </h3>
                  <dl class="legend-terms">
                    <dt>
                      M:
                    </dt>
                    <dd>
                      Memory
                    </dd>
                    <dt>
                      C:
                    </dt>
                    <dd>
                      CPU
                    </dd>
                  </dl>
                </div>
                <div class="legend">
                  <h3 class="legend-label">
                    Allocation Status
                  </h3>
                  <dl class="legend-terms">
                    <div class="legend-term">
                      <dt>
                        <span
                          class="color-swatch is-wide running"
                          title="Running"
                        ></span>
                      </dt>
                      <dd>
                        Running
                      </dd>
                    </div>
                    <div class="legend-term">
                      <dt>
                        <span
                          class="color-swatch is-wide pending"
                          title="Starting"
                        ></span>
                      </dt>
                      <dd>
                        Starting
                      </dd>
                    </div>
                  </dl>
                </div>
              </div>
            </div>
            <div class="boxed-section">
              <div data-test-info-panel-title class="boxed-section-head">
                {{#if @controller.activeNode}}
                  Client
                {{else if @controller.activeAllocation}}
                  Allocation
                {{else}}
                  Cluster
                {{/if}}
                Details
              </div>
              <div data-test-info-panel class="boxed-section-body">
                {{#if @controller.activeNode}}
                  {{#let @controller.activeNode.node as |node|}}
                    <div class="dashboard-metric">
                      <p data-test-allocations class="metric">
                        {{@controller.activeNode.allocations.length}}
                        <span class="metric-label">
                          Allocations
                        </span>
                      </p>
                    </div>
                    <div class="dashboard-metric">
                      <h3 class="pair">
                        <strong>
                          Client:
                        </strong>
                        <LinkTo
                          data-test-client-link
                          @route="clients.client"
                          @model={{node.id}}
                        >
                          {{node.shortId}}
                        </LinkTo>
                      </h3>
                      <p data-test-name class="minor-pair">
                        <strong>
                          Name:
                        </strong>
                        {{node.name}}
                      </p>
                      <p data-test-address class="minor-pair">
                        <strong>
                          Address:
                        </strong>
                        {{node.httpAddr}}
                      </p>
                      <p data-test-status class="minor-pair">
                        <strong>
                          Status:
                        </strong>
                        {{node.status}}
                      </p>
                    </div>
                    <div class="dashboard-metric">
                      <h3 class="pair">
                        <strong>
                          Draining?
                        </strong>
                        <span
                          data-test-draining
                          class={{if node.isDraining "status-text is-info"}}
                        >
                          {{if node.isDraining "Yes" "No"}}
                        </span>
                      </h3>
                      <h3 class="pair">
                        <strong>
                          Eligible?
                        </strong>
                        <span
                          data-test-eligible
                          class={{unless
                            node.isEligible
                            "status-text is-warning"
                          }}
                        >
                          {{if node.isEligible "Yes" "No"}}
                        </span>
                      </h3>
                    </div>
                    <div class="dashboard-metric with-divider">
                      <p class="metric">
                        {{@controller.nodeUtilization.totalMemoryFormatted}}
                        <span class="metric-units">
                          {{@controller.nodeUtilization.totalMemoryUnits}}
                        </span>
                        <span class="metric-label">
                          of memory
                        </span>
                      </p>
                      <div class="columns graphic">
                        <div class="column">
                          <div class="inline-chart">
                            <progress
                              data-test-memory-progress-bar
                              class="progress is-danger is-small"
                              value={{@controller.nodeUtilization.reservedMemoryPercent}}
                              max="1"
                            >
                              {{@controller.nodeUtilization.reservedMemoryPercent}}
                            </progress>
                          </div>
                        </div>
                        <div class="column is-minimum">
                          <span class="nowrap" data-test-percentage>
                            {{formatPercentage
                              @controller.nodeUtilization.reservedMemoryPercent
                              total=1
                            }}
                          </span>
                        </div>
                      </div>
                      <div class="annotation" data-test-memory-absolute-value>
                        <strong>
                          {{formatScheduledBytes
                            @controller.nodeUtilization.totalReservedMemory
                          }}
                        </strong>
                        /
                        {{formatScheduledBytes
                          @controller.nodeUtilization.totalMemory
                        }}
                        reserved
                      </div>
                    </div>
                    <div class="dashboard-metric">
                      <p class="metric">
                        {{@controller.nodeUtilization.totalCPU}}
                        <span class="metric-units">
                          MHz
                        </span>
                        <span class="metric-label">
                          of CPU
                        </span>
                      </p>
                      <div class="columns graphic">
                        <div class="column">
                          <div class="inline-chart" data-test-percentage-bar>
                            <progress
                              data-test-cpu-progress-bar
                              class="progress is-info is-small"
                              value={{@controller.nodeUtilization.reservedCPUPercent}}
                              max="1"
                            >
                              {{@controller.nodeUtilization.reservedCPUPercent}}
                            </progress>
                          </div>
                        </div>
                        <div class="column is-minimum">
                          <span class="nowrap" data-test-percentage>
                            {{formatPercentage
                              @controller.nodeUtilization.reservedCPUPercent
                              total=1
                            }}
                          </span>
                        </div>
                      </div>
                      <div class="annotation" data-test-cpu-absolute-value>
                        <strong>
                          {{formatScheduledHertz
                            @controller.nodeUtilization.totalReservedCPU
                          }}
                        </strong>
                        /
                        {{formatScheduledHertz
                          @controller.nodeUtilization.totalCPU
                        }}
                        reserved
                      </div>
                    </div>
                  {{/let}}
                {{else if @controller.activeAllocation}}
                  <div class="dashboard-metric">
                    <h3 class="pair">
                      <strong>
                        Allocation:
                      </strong>
                      <LinkTo
                        data-test-id
                        @route="allocations.allocation"
                        @model={{@controller.activeAllocation}}
                        class="is-primary"
                      >
                        {{@controller.activeAllocation.shortId}}
                      </LinkTo>
                    </h3>
                    <p data-test-sibling-allocs class="minor-pair">
                      <strong>
                        Sibling Allocations:
                      </strong>
                      {{@controller.siblingAllocations.length}}
                    </p>
                    <p data-test-unique-placements class="minor-pair">
                      <strong>
                        Unique Client Placements:
                      </strong>
                      {{@controller.uniqueActiveAllocationNodes.length}}
                    </p>
                  </div>
                  <div class="dashboard-metric with-divider">
                    <h3 class="pair">
                      <strong>
                        Job:
                      </strong>
                      <LinkTo
                        data-test-job
                        @route="jobs.job"
                        @model={{@controller.activeAllocation.job}}
                      >
                        {{@controller.activeAllocation.job.name}}
                      </LinkTo>
                      <span class="is-faded" data-test-task-group>
                        /
                        {{@controller.activeAllocation.taskGroupName}}
                      </span>
                    </h3>
                    <p class="minor-pair">
                      <strong>
                        Type:
                      </strong>
                      {{@controller.activeAllocation.job.type}}
                    </p>
                    <p class="minor-pair">
                      <strong>
                        Priority:
                      </strong>
                      {{@controller.activeAllocation.job.priority}}
                    </p>
                  </div>
                  <div class="dashboard-metric with-divider">
                    <h3 class="pair">
                      <strong>
                        Client:
                      </strong>
                      <LinkTo
                        data-test-client
                        @route="clients.client"
                        @model={{@controller.activeAllocation.node.id}}
                      >
                        {{@controller.activeAllocation.node.shortId}}
                      </LinkTo>
                    </h3>
                    <p class="minor-pair">
                      <strong>
                        Name:
                      </strong>
                      {{@controller.activeAllocation.node.name}}
                    </p>
                    <p class="minor-pair">
                      <strong>
                        Address:
                      </strong>
                      {{@controller.activeAllocation.node.httpAddr}}
                    </p>
                  </div>
                  <div class="dashboard-metric with-divider">
                    <PrimaryMetricAllocation
                      @allocation={{@controller.activeAllocation}}
                      @metric="memory"
                      class="is-short"
                    />
                  </div>
                  <div class="dashboard-metric">
                    <PrimaryMetricAllocation
                      @allocation={{@controller.activeAllocation}}
                      @metric="cpu"
                      class="is-short"
                    />
                  </div>
                {{else}}
                  <div class="columns is-flush">
                    <div class="dashboard-metric column">
                      <p data-test-node-count class="metric justify">
                        {{@model.nodes.length}}
                        <span class="metric-label">
                          Clients
                        </span>
                      </p>
                    </div>
                    <div class="dashboard-metric column">
                      <p data-test-alloc-count class="metric justify">
                        {{@controller.scheduledAllocations.length}}
                        <span class="metric-label">
                          Allocations
                        </span>
                      </p>
                    </div>
                    <div class="dashboard-metric column">
                      <p data-test-node-pool-count class="metric justify">
                        {{@model.nodePools.length}}
                        <span class="metric-label">
                          Node Pools
                        </span>
                      </p>
                    </div>
                  </div>
                  <div class="dashboard-metric with-divider">
                    <p class="metric">
                      {{@controller.totalMemoryFormatted}}
                      <span class="metric-units">
                        {{@controller.totalMemoryUnits}}
                      </span>
                      <span class="metric-label">
                        of memory
                      </span>
                    </p>
                    <div class="columns graphic">
                      <div class="column">
                        <div class="inline-chart" data-test-percentage-bar>
                          <progress
                            data-test-memory-progress-bar
                            class="progress is-danger is-small"
                            value={{@controller.reservedMemoryPercent}}
                            max="1"
                          >
                            {{@controller.reservedMemoryPercent}}
                          </progress>
                        </div>
                      </div>
                      <div class="column is-minimum">
                        <span class="nowrap" data-test-memory-percentage>
                          {{formatPercentage
                            @controller.reservedMemoryPercent
                            total=1
                          }}
                        </span>
                      </div>
                    </div>
                    <div class="annotation" data-test-memory-absolute-value>
                      <strong>
                        {{formatBytes @controller.totalReservedMemory}}
                      </strong>
                      /
                      {{formatBytes @controller.totalMemory}}
                      reserved
                    </div>
                  </div>
                  <div class="dashboard-metric">
                    <p class="metric">
                      {{@controller.totalCPUFormatted}}
                      <span class="metric-units">
                        {{@controller.totalCPUUnits}}
                      </span>
                      <span class="metric-label">
                        of CPU
                      </span>
                    </p>
                    <div class="columns graphic">
                      <div class="column">
                        <div class="inline-chart" data-test-percentage-bar>
                          <progress
                            data-test-cpu-progress-bar
                            class="progress is-info is-small"
                            value={{@controller.reservedCPUPercent}}
                            max="1"
                          >
                            {{@controller.reservedCPUPercent}}
                          </progress>
                        </div>
                      </div>
                      <div class="column is-minimum">
                        <span class="nowrap" data-test-cpu-percentage>
                          {{formatPercentage
                            @controller.reservedCPUPercent
                            total=1
                          }}
                        </span>
                      </div>
                    </div>
                    <div class="annotation" data-test-cpu-absolute-value>
                      <strong>
                        {{formatHertz @controller.totalReservedCPU}}
                      </strong>
                      /
                      {{formatHertz @controller.totalCPU}}
                      reserved
                    </div>
                  </div>
                {{/if}}
              </div>
            </div>
          </div>
          <div class="column">
            <div class="toolbar">
              <div class="toolbar-item">
                {{#if @model.nodes.length}}
                  <SearchBox
                    @inputClass="node-search"
                    @searchTerm={{@controller.searchTerm}}
                    @onChange={{@controller.setSearchTerm}}
                    @placeholder="Search clients..."
                  />
                {{/if}}
              </div>
              <div class="toolbar-item is-right-aligned is-mobile-full-width">
                <div class="button-bar">
                  <MultiSelectDropdown
                    data-test-node-pool-facet
                    @label="Node Pool"
                    @options={{@controller.optionsNodePool}}
                    @selection={{@controller.selectionNodePool}}
                    @onSelect={{@controller.selectNodePool}}
                  />
                  <MultiSelectDropdown
                    data-test-datacenter-facet
                    @label="Datacenter"
                    @options={{@controller.optionsDatacenter}}
                    @selection={{@controller.selectionDatacenter}}
                    @onSelect={{@controller.selectDatacenter}}
                  />
                  <MultiSelectDropdown
                    data-test-class-facet
                    @label="Class"
                    @options={{@controller.optionsClass}}
                    @selection={{@controller.selectionClass}}
                    @onSelect={{@controller.selectClass}}
                  />
                  <MultiSelectDropdown
                    data-test-state-facet
                    @label="State"
                    @options={{@controller.optionsState}}
                    @selection={{@controller.selectionState}}
                    @onSelect={{@controller.selectState}}
                  />
                  <MultiSelectDropdown
                    data-test-version-facet
                    @label="Version"
                    @options={{@controller.optionsVersion}}
                    @selection={{@controller.selectionVersion}}
                    @onSelect={{@controller.selectVersion}}
                  />
                </div>
              </div>
            </div>
            <TopoViz
              @nodes={{@controller.filteredNodes}}
              @allocations={{@model.allocations}}
              @onAllocationSelect={{@controller.setAllocation}}
              @onNodeSelect={{@controller.setNode}}
              @onDataError={{@controller.handleTopoVizDataError}}
              @filters={{hash
                search=@controller.searchTerm
                clientState=@controller.selectionState
                clientVersion=@controller.selectionVersion
              }}
            />
          </div>
        </div>
      {{/if}}
    </section>
  </PageLayout>
</template>
