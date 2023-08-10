/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { htmlSafe } from '@ember/template';
import { action, get } from '@ember/object';
import styleString from 'nomad-ui/utils/properties/glimmer-style-string';

export default class ChartPrimitiveVAnnotations extends Component {
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

    let sortedAnnotations = annotations.sortBy(prop).reverse();

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

  @action
  selectAnnotation(annotation) {
    if (this.args.annotationClick) this.args.annotationClick(annotation);
  }
}
