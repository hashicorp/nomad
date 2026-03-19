/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { guidFor } from '@ember/object/internals';
import { LinkTo } from '@ember/routing';
import {
  HdsIcon,
  HdsTooltipButton,
} from '@hashicorp/design-system-components/components';
import { on } from '@ember/modifier';
import { fn } from '@ember/helper';
import { eq, not, or } from 'ember-truth-helpers';

import didInsert from '@ember/render-modifiers/modifiers/did-insert';

import didUpdate from '@ember/render-modifiers/modifiers/did-update';
import hdsTooltip from '@hashicorp/design-system-components/modifiers/hds-tooltip';
import windowResize from 'nomad-ui/modifiers/window-resize';
import formatScheduledBytes from 'nomad-ui/helpers/format-scheduled-bytes';
import formatScheduledHertz from 'nomad-ui/helpers/format-scheduled-hertz';

export default class TopoVizNode extends Component {
  @tracked data = { cpu: [], memory: [] };
  @tracked dimensionsWidth = 0;
  @tracked padding = 5;
  @tracked activeAllocation = null;

  get height() {
    return this.args.heightScale
      ? this.args.heightScale(this.args.node.memory)
      : 15;
  }

  get labelHeight() {
    return this.height / 2;
  }

  get paddingLeft() {
    const labelWidth = 20;
    return this.padding + labelWidth;
  }

  // Since strokes are placed centered on the perimeter of fills, The width of the stroke needs to be removed from
  // the height of the fill to match unstroked height and avoid clipping.
  get selectedHeight() {
    return this.height - 1;
  }

  // Since strokes are placed centered on the perimeter of fills, half the width of the stroke needs to be added to
  // the yOffset to match heights with unstroked shapes.
  get selectedYOffset() {
    return this.height + 2.5;
  }

  get yOffset() {
    return this.height + 2;
  }

  get maskHeight() {
    return this.height + this.yOffset;
  }

  get totalHeight() {
    return this.maskHeight + this.padding * 2;
  }

  get maskId() {
    return `topo-viz-node-mask-${guidFor(this)}`;
  }

  get count() {
    return this.allocations.length;
  }

  get allocations() {
    // Sort by the delta between memory and cpu percent. This creates the least amount of
    // drift between the positional alignment of an alloc's cpu and memory representations.
    return this.args.node.allocations
      .filterBy('allocation.isScheduled')
      .sort((a, b) => {
        const deltaA = Math.abs(a.memoryPercent - a.cpuPercent);
        const deltaB = Math.abs(b.memoryPercent - b.cpuPercent);
        return deltaA - deltaB;
      });
  }

  reloadNode = async () => {
    if (this.args.node.isPartial) {
      await this.args.node.reload();
      this.data = this.computeData(this.dimensionsWidth);
    }
  };

  render = (svg) => {
    this.dimensionsWidth = svg.clientWidth - this.padding - this.paddingLeft;
    this.data = this.computeData(this.dimensionsWidth);
  };

  updateRender = (svg) => {
    // Only update all data when the width changes
    const newWidth = svg.clientWidth - this.padding - this.paddingLeft;
    if (newWidth !== this.dimensionsWidth) {
      this.dimensionsWidth = newWidth;
      this.data = this.computeData(this.dimensionsWidth);
    }
  };

  highlightAllocation = (allocation, { target }) => {
    this.activeAllocation = allocation;
    this.args.onAllocationFocus &&
      this.args.onAllocationFocus(allocation, target);
  };

  allocationBlur = () => {
    this.args.onAllocationBlur && this.args.onAllocationBlur();
  };

  clearHighlight = () => {
    this.activeAllocation = null;
  };

  selectNode = () => {
    if (this.args.isDense && this.args.onNodeSelect) {
      this.args.onNodeSelect(this.args.node.isSelected ? null : this.args.node);
    }
  };

  selectAllocation = (allocation) => {
    if (this.args.onAllocationSelect) this.args.onAllocationSelect(allocation);
  };

  containsActiveTaskGroup() {
    return this.args.node.allocations.some(
      (allocation) =>
        allocation.taskGroupName === this.args.activeTaskGroup &&
        allocation.belongsTo('job').id() === this.args.activeJobId,
    );
  }

  computeData(width) {
    const allocations = this.allocations;
    let cpuOffset = 0;
    let memoryOffset = 0;

    const cpu = [];
    const memory = [];
    for (const allocation of allocations) {
      const { cpuPercent, memoryPercent, isSelected } = allocation;
      const isFirst = allocation === allocations[0];

      let cpuWidth = cpuPercent * width - 1;
      let memoryWidth = memoryPercent * width - 1;
      if (isFirst) {
        cpuWidth += 0.5;
        memoryWidth += 0.5;
      }
      if (isSelected) {
        cpuWidth--;
        memoryWidth--;
      }

      cpu.push({
        allocation,
        offset: cpuOffset * 100,
        percent: cpuPercent * 100,
        width: Math.max(cpuWidth, 0),
        x: cpuOffset * width + (isFirst ? 0 : 0.5) + (isSelected ? 0.5 : 0),
        className: allocation.allocation.clientStatus,
      });
      memory.push({
        allocation,
        offset: memoryOffset * 100,
        percent: memoryPercent * 100,
        width: Math.max(memoryWidth, 0),
        x: memoryOffset * width + (isFirst ? 0 : 0.5) + (isSelected ? 0.5 : 0),
        className: allocation.allocation.clientStatus,
      });

      cpuOffset += cpuPercent;
      memoryOffset += memoryPercent;
    }

    const cpuRemainder = {
      x: cpuOffset * width + 0.5,
      width: Math.max(width - cpuOffset * width, 0),
    };
    const memoryRemainder = {
      x: memoryOffset * width + 0.5,
      width: Math.max(width - memoryOffset * width, 0),
    };

    return {
      cpu,
      memory,
      cpuRemainder,
      memoryRemainder,
      cpuLabel: { x: -this.paddingLeft / 2, y: this.height / 2 + this.yOffset },
      memoryLabel: { x: -this.paddingLeft / 2, y: this.height / 2 },
    };
  }

  <template>
    <div
      data-test-topo-viz-node
      class="topo-viz-node {{unless this.allocations.length 'is-empty'}}"
      {{didInsert this.reloadNode}}
    >
      {{#unless @isDense}}
        <p data-test-label class="label">
          {{#if @node.node.isDraining}}
            <HdsTooltipButton
              data-test-status-icon
              @text="Client is draining"
              aria-label="Client is draining"
            >
              <HdsIcon @name="clock" @isInline={{true}} />
            </HdsTooltipButton>
          {{else if (not @node.node.isEligible)}}
            <HdsTooltipButton
              data-test-status-icon
              @text="Client is ineligible"
              aria-label="Client is ineligible"
            >
              <HdsIcon @name="lock" @isInline={{true}} />
            </HdsTooltipButton>
          {{/if}}
          <LinkTo
            @route="clients.client"
            @model={{@node.node.id}}
            {{hdsTooltip "Node Name"}}
          >{{@node.node.name}}</LinkTo>
          <HdsTooltipButton
            @text="Number of Allocations"
            class="bumper-left"
          >{{this.count}} Allocs</HdsTooltipButton>
          {{#if @node.node.nodePool}}
            <HdsTooltipButton
              @text="Node Pool"
              class="bumper-left is-faded"
            >{{@node.node.nodePool}}</HdsTooltipButton>
          {{/if}}
          {{#if @node.memory}}
            <HdsTooltipButton
              @text="Node Memory"
              class="bumper-left is-faded"
            >{{formatScheduledBytes
                @node.memory
                start="MiB"
              }}</HdsTooltipButton>
          {{/if}}
          {{#if @node.cpu}}
            <HdsTooltipButton
              @text="Node CPU"
              class="bumper-left is-faded"
            >{{formatScheduledHertz @node.cpu}}</HdsTooltipButton>
          {{/if}}
          {{#if @node.node.status}}
            <HdsTooltipButton
              @text="Node Status"
              class="bumper-left is-faded"
            >{{@node.node.status}}</HdsTooltipButton>
          {{/if}}
          {{#if @node.node.version}}
            <HdsTooltipButton
              @text="Nomad Version"
              class="bumper-left is-faded"
            >{{@node.node.version}}</HdsTooltipButton>
          {{/if}}
        </p>
      {{/unless}}
      <svg
        data-test-topo-node-svg
        class="chart"
        height="{{this.totalHeight}}px"
        {{didInsert this.render}}
        {{didUpdate this.updateRender}}
        {{windowResize this.render}}
        {{on "mouseout" this.allocationBlur}}
      >
        <defs>
          <clipPath id="{{this.maskId}}">
            <rect
              class="mask"
              x="0"
              y="0"
              width="{{this.dimensionsWidth}}px"
              height="{{this.maskHeight}}px"
              rx="2px"
              ry="2px"
            />
          </clipPath>
        </defs>
        <rect
          data-test-node-background
          class="node-background
            {{if @node.isSelected 'is-selected'}}
            {{if @isDense 'is-interactive'}}"
          width="100%"
          height="{{this.totalHeight}}px"
          rx="2px"
          ry="2px"
          {{on "click" this.selectNode}}
        />
        {{#if this.allocations.length}}
          <g
            class="dimensions {{if this.activeAllocation 'is-active'}}"
            transform="translate({{this.paddingLeft}},{{this.padding}})"
            width="{{this.dimensionsWidth}}px"
            height="{{this.maskHeight}}px"
            pointer-events="all"
            {{on "mouseleave" this.clearHighlight}}
          >
            <g class="memory">
              {{#if this.data.memoryLabel}}
                <text
                  class="label"
                  aria-label="Memory"
                  transform="translate({{this.data.memoryLabel.x}},{{this.data.memoryLabel.y}})"
                >M</text>
              {{/if}}
              {{#if this.data.memoryRemainder}}
                <rect
                  class="dimension-background"
                  x="{{this.data.memoryRemainder.x}}px"
                  width="{{this.data.memoryRemainder.width}}px"
                  height="{{this.height}}px"
                />
              {{/if}}
              {{#each this.data.memory key="allocation.id" as |memory|}}
                <g
                  data-test-memory-rect="{{memory.allocation.allocation.id}}"
                  class="bar
                    {{memory.className}}
                    {{if
                      (eq this.activeAllocation memory.allocation)
                      'is-active'
                    }}
                    {{if memory.allocation.isSelected 'is-selected'}}"
                  clip-path="url(#{{this.maskId}})"
                  data-allocation-id="{{memory.allocation.allocation.id}}"
                  {{on
                    "mouseenter"
                    (fn this.highlightAllocation memory.allocation)
                  }}
                  {{on "click" (fn this.selectAllocation memory.allocation)}}
                >
                  <rect
                    width="{{memory.width}}px"
                    height="{{if
                      memory.allocation.isSelected
                      this.selectedHeight
                      this.height
                    }}px"
                    x="{{memory.x}}px"
                    y="{{if memory.allocation.isSelected 0.5 0}}px"
                    class="layer-0"
                  />
                  {{#if
                    (or
                      (eq memory.className "starting")
                      (eq memory.className "pending")
                    )
                  }}
                    <rect
                      width="{{memory.width}}px"
                      height="{{if
                        memory.allocation.isSelected
                        this.selectedHeight
                        this.height
                      }}px"
                      x="{{memory.x}}px"
                      y="{{if memory.allocation.isSelected 0.5 0}}px"
                      class="layer-1"
                    />
                  {{/if}}
                </g>
              {{/each}}
            </g>
            <g class="cpu">
              {{#if this.data.cpuLabel}}
                <text
                  class="label"
                  aria-label="CPU"
                  transform="translate({{this.data.cpuLabel.x}},{{this.data.cpuLabel.y}})"
                >C</text>
              {{/if}}
              {{#if this.data.cpuRemainder}}
                <rect
                  class="dimension-background"
                  x="{{this.data.cpuRemainder.x}}px"
                  y="{{this.yOffset}}px"
                  width="{{this.data.cpuRemainder.width}}px"
                  height="{{this.height}}px"
                />
              {{/if}}
              {{#each this.data.cpu key="allocation.id" as |cpu|}}
                <g
                  data-test-cpu-rect="{{cpu.allocation.allocation.id}}"
                  class="bar
                    {{cpu.className}}
                    {{if (eq this.activeAllocation cpu.allocation) 'is-active'}}
                    {{if cpu.allocation.isSelected 'is-selected'}}"
                  clip-path="url(#{{this.maskId}})"
                  data-allocation-id="{{cpu.allocation.allocation.id}}"
                  {{on
                    "mouseenter"
                    (fn this.highlightAllocation cpu.allocation)
                  }}
                  {{on "click" (fn this.selectAllocation cpu.allocation)}}
                >
                  <rect
                    width="{{cpu.width}}px"
                    height="{{if
                      cpu.allocation.isSelected
                      this.selectedHeight
                      this.height
                    }}px"
                    x="{{cpu.x}}px"
                    y="{{if
                      cpu.allocation.isSelected
                      this.selectedYOffset
                      this.yOffset
                    }}px"
                    class="layer-0"
                  />
                  {{#if
                    (or
                      (eq cpu.className "starting") (eq cpu.className "pending")
                    )
                  }}
                    <rect
                      width="{{cpu.width}}px"
                      height="{{if
                        cpu.allocation.isSelected
                        this.selectedHeight
                        this.height
                      }}px"
                      x="{{cpu.x}}px"
                      y="{{if
                        cpu.allocation.isSelected
                        this.selectedYOffset
                        this.yOffset
                      }}px"
                      class="layer-1"
                    />
                  {{/if}}
                </g>
              {{/each}}
            </g>
          </g>
        {{else}}
          <g class="empty-text"><text data-test-empty-message>Empty Client</text></g>
        {{/if}}
      </svg>
    </div>
  </template>
}
