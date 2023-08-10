/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { action } from '@ember/object';
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import d3 from 'd3';
import { matchesState } from 'ember-statecharts';

export default class Detail extends Component {
  get statechart() {
    return this.args.statechart;
  }

  @matchesState({ sidebar: 'open' })
  isSideBarOpen;

  @matchesState({ sidebar: { open: 'success' } })
  isSuccess;

  @matchesState({ sidebar: { open: 'busy' } })
  isLoading;

  @matchesState({ sidebar: { open: 'error' } })
  isError;

  @tracked width = null;
  @tracked height = null;

  @action
  handleResize({ target: { scrollWidth: width, scrollHeight: height } }) {
    if (width === this.width || height === this.height) return;
    this.height = height;
    this.width = width;
  }

  get currentEvalDetail() {
    return this.statechart.state.context.evaluation;
  }

  get hierarchy() {
    try {
      const data = this.currentEvalDetail?.relatedEvals;

      if (data) {
        return d3
          .stratify()
          .id((d) => {
            return d.id;
          })
          .parentId((d) => d.previousEval)([
          ...data.toArray(),
          this.currentEvalDetail,
        ]);
      }
    } catch (e) {
      console.error(`\n\nRelated Evaluation Error:   ${e.message}`);
    }
    return null;
  }

  get descendentsMap() {
    return this.hierarchy
      ?.descendants()
      .map((d) => d.children)
      .compact();
  }

  get parentEvaluation() {
    return this.hierarchy?.data;
  }

  get error() {
    return this.statechart.state.context.error;
  }

  @action
  closeSidebar() {
    return this.statechart.send('MODAL_CLOSE');
  }

  keyCommands = [
    {
      label: 'Close Evaluations Sidebar',
      pattern: ['Escape'],
      action: () => this.closeSidebar(),
    },
  ];
}
