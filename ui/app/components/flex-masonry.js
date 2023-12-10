/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { next } from '@ember/runloop';
import { action } from '@ember/object';
import { minIndex, max } from 'd3-array';

export default class FlexMasonry extends Component {
  @tracked element = null;

  @action
  captureElement(element) {
    this.element = element;
  }

  @action
  reflow() {
    next(() => {
      // There's nothing to do if there is no element
      if (!this.element) return;

      const items = this.element.querySelectorAll(
        ':scope > .flex-masonry-item'
      );

      // Clear out specified order and flex-basis values in case this was once a multi-column layout
      if (this.args.columns === 1 || !this.args.columns) {
        for (let item of items) {
          item.style.flexBasis = null;
          item.style.order = null;
        }
        this.element.style.maxHeight = null;
        return;
      }

      const columns = new Array(this.args.columns).fill(null).map(() => ({
        height: 0,
        elements: [],
      }));

      // First pass: assign each element to a column based on the running heights of each column
      for (let item of items) {
        const styles = window.getComputedStyle(item);
        const marginTop = parseFloat(styles.marginTop);
        const marginBottom = parseFloat(styles.marginBottom);
        const height = item.clientHeight;

        // Pick the shortest column accounting for margins
        const column = columns[minIndex(columns, (c) => c.height)];

        // Add the new element's height to the column height
        column.height += marginTop + height + marginBottom;
        column.elements.push(item);
      }

      // Second pass: assign an order to each element based on their column and position in the column
      columns
        .mapBy('elements')
        .flat()
        .forEach((dc, index) => {
          dc.style.order = index;
        });

      // Guarantee column wrapping as predicted (if the first item of a column is shorter than the difference
      // beteen the height of the column and the previous column, then flexbox will naturally place the first
      // item at the end of the previous column).
      columns.forEach((column, index) => {
        const nextHeight =
          index < columns.length - 1 ? columns[index + 1].height : 0;
        const item = column.elements.lastObject;
        if (item) {
          item.style.flexBasis =
            item.clientHeight + Math.max(0, nextHeight - column.height) + 'px';
        }
      });

      // Set the max height of the container to the height of the tallest column
      this.element.style.maxHeight = max(columns.mapBy('height')) + 1 + 'px';
    });
  }
}
