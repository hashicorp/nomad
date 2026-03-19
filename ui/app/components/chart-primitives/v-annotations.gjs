/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { htmlSafe } from '@ember/template';
import { get } from '@ember/object';
import { on } from '@ember/modifier';
import { fn } from '@ember/helper';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import styleString from 'nomad-ui/utils/properties/glimmer-style-string';

const iconFor = {
  error: 'x-circle-fill',
  info: 'info-fill',
};

const iconClassFor = {
  error: 'is-danger',
  info: '',
};

export default class ChartPrimitiveVAnnotations extends Component {
  @styleString
  get chartAnnotationsStyle() {
    return {
      height: this.args.height,
    };
  }

  get processed() {
    const { scale, prop, annotations, timeseries, format } = this.args;

    if (!annotations || !annotations.length) return null;

    let sortedAnnotations = annotations.sortBy(prop);
    if (timeseries) {
      sortedAnnotations = sortedAnnotations.reverse();
    }

    let prevX = 0;
    let prevHigh = false;
    return sortedAnnotations.map((annotation) => {
      const x = scale(annotation[prop]);
      if (prevX && !prevHigh && Math.abs(x - prevX) < 30) {
        prevHigh = true;
      } else if (prevHigh) {
        prevHigh = false;
      }
      const y = prevHigh ? -15 : 0;
      const formattedX = format(timeseries)(annotation[prop]);

      prevX = x;
      return {
        annotation,
        style: htmlSafe(`transform:translate(${x}px,${y}px)`),
        icon: iconFor[annotation.type],
        iconClass: iconClassFor[annotation.type],
        staggerClass: prevHigh ? 'is-staggered' : '',
        label: `${annotation.type} event at ${formattedX}`,
        isActive: this.annotationIsActive(annotation),
      };
    });
  }

  annotationIsActive(annotation) {
    const { key, activeAnnotation } = this.args;
    if (!activeAnnotation) return false;

    if (key) return get(annotation, key) === get(activeAnnotation, key);
    return annotation === activeAnnotation;
  }

  selectAnnotation = (annotation) => {
    if (this.args.annotationClick) this.args.annotationClick(annotation);
  };

  <template>
    <div
      data-test-annotations
      class="line-chart-annotations"
      style={{this.chartAnnotationsStyle}}
      ...attributes
    >
      {{#each this.processed key=@key as |annotation|}}
        <div
          data-test-annotation
          class="chart-vertical-annotation
            {{annotation.iconClass}}
            {{annotation.staggerClass}}"
          style={{annotation.style}}
        >
          <button
            type="button"
            title={{annotation.label}}
            class="indicator {{if annotation.isActive 'is-active'}}"
            {{on "click" (fn this.selectAnnotation annotation.annotation)}}
          >
            <HdsIcon @name={{annotation.icon}} @isInline={{true}} />
          </button>
          <div class="line" />
        </div>
      {{/each}}
    </div>
  </template>
}
