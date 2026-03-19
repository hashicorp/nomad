/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { htmlSafe } from '@ember/template';
import { get } from '@ember/object';
import { on } from '@ember/modifier';
import { fn } from '@ember/helper';
import styleString from 'nomad-ui/utils/properties/glimmer-style-string';

export default class ChartPrimitiveHAnnotations extends Component {
  @styleString
  get chartAnnotationsStyle() {
    return {
      width: this.args.width,
      left: this.args.left,
    };
  }

  get processed() {
    const { scale, prop, annotations, format, labelProp } = this.args;

    if (!annotations || !annotations.length) return null;

    const sortedAnnotations = annotations.sortBy(prop).reverse();

    return sortedAnnotations.map((annotation) => {
      const y = scale(annotation[prop]);
      const x = 0;
      const formattedY = format()(annotation[prop]);

      return {
        annotation,
        style: htmlSafe(`transform:translate(${x}px,${y}px)`),
        label: annotation[labelProp],
        a11yLabel: `${annotation[labelProp]} at ${formattedY}`,
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
          class="chart-horizontal-annotation"
          style={{annotation.style}}
        >
          <button
            type="button"
            title={{annotation.a11yLabel}}
            class="indicator {{if annotation.isActive 'is-active'}}"
            {{on "click" (fn this.selectAnnotation annotation.annotation)}}
          >
            {{annotation.label}}
          </button>
          <div class="line" />
        </div>
      {{/each}}
    </div>
  </template>
}
